package storage

import (
	"context"
	"database/sql"
	"os"
	"time"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
)

type DB struct {
	SQL *sql.DB
}

func Connect() (*DB, error) {
	cs := os.Getenv("MSSQL_CONN")
	if cs == "" {
		cs = "sqlserver://sa:YourStrong!Passw0rd@mssql:1433?database=events&encrypt=disable"
	}
	db, err := sql.Open("sqlserver", cs)
	if err != nil { return nil, err }
	// Try ping; if database missing, create it by connecting to master.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		masterCS := cs
		masterCS = strings.ReplaceAll(masterCS, "database=events", "database=master")
		mdb, merr := sql.Open("sqlserver", masterCS)
		if merr != nil { return nil, merr }
		_, _ = mdb.Exec("IF DB_ID('events') IS NULL CREATE DATABASE events;")
		_ = mdb.Close()
		// reopen original DB
		db, err = sql.Open("sqlserver", cs)
		if err != nil { return nil, err }
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return &DB{SQL: db}, nil
}

func (d *DB) Init(ctx context.Context) error {
	schema := `
IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[users]') AND type in (N'U'))
BEGIN
CREATE TABLE dbo.users (
	user_id NVARCHAR(64) PRIMARY KEY,
	name NVARCHAR(255) NOT NULL,
	email NVARCHAR(255) NOT NULL,
	updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
END;

IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[orders]') AND type in (N'U'))
BEGIN
CREATE TABLE dbo.orders (
	order_id NVARCHAR(64) PRIMARY KEY,
	user_id NVARCHAR(64) NOT NULL,
	amount DECIMAL(18,2) NOT NULL,
	updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
CREATE INDEX IX_orders_user ON dbo.orders(user_id);
END;

IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[payments]') AND type in (N'U'))
BEGIN
CREATE TABLE dbo.payments (
	payment_id NVARCHAR(64) PRIMARY KEY,
	order_id NVARCHAR(64) NOT NULL,
	status NVARCHAR(32) NOT NULL,
	amount DECIMAL(18,2) NOT NULL,
	updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
END;

IF NOT EXISTS (SELECT * FROM sys.objects WHERE object_id = OBJECT_ID(N'[dbo].[inventory]') AND type in (N'U'))
BEGIN
CREATE TABLE dbo.inventory (
	sku NVARCHAR(64) PRIMARY KEY,
	qty INT NOT NULL DEFAULT 0,
	updated_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME()
);
END;
`
	_, err := d.SQL.ExecContext(ctx, schema)
	return err
}

// Idempotent upserts
func (d *DB) UpsertUser(ctx context.Context, id, name, email string) error {
	q := `MERGE dbo.users AS t USING (SELECT @p1 AS user_id, @p2 AS name, @p3 AS email) AS s
	ON (t.user_id=s.user_id)
	WHEN MATCHED THEN UPDATE SET name=s.name, email=s.email, updated_at=SYSUTCDATETIME()
	WHEN NOT MATCHED THEN INSERT(user_id,name,email,updated_at) VALUES(s.user_id,s.name,s.email,SYSUTCDATETIME());`
	_, err := d.SQL.ExecContext(ctx, q, id, name, email)
	return err
}

func (d *DB) UpsertOrder(ctx context.Context, id, userID string, amount float64) error {
	q := `MERGE dbo.orders AS t USING (SELECT @p1 AS order_id, @p2 AS user_id, @p3 AS amount) AS s
	ON (t.order_id=s.order_id)
	WHEN MATCHED THEN UPDATE SET user_id=s.user_id, amount=s.amount, updated_at=SYSUTCDATETIME()
	WHEN NOT MATCHED THEN INSERT(order_id,user_id,amount,updated_at) VALUES(s.order_id,s.user_id,s.amount,SYSUTCDATETIME());`
	_, err := d.SQL.ExecContext(ctx, q, id, userID, amount)
	return err
}

func (d *DB) UpsertPayment(ctx context.Context, id, orderID, status string, amount float64) error {
	q := `MERGE dbo.payments AS t USING (SELECT @p1 AS payment_id, @p2 AS order_id, @p3 AS status, @p4 AS amount) AS s
	ON (t.payment_id=s.payment_id)
	WHEN MATCHED THEN UPDATE SET order_id=s.order_id, status=s.status, amount=s.amount, updated_at=SYSUTCDATETIME()
	WHEN NOT MATCHED THEN INSERT(payment_id,order_id,status,amount,updated_at) VALUES(s.payment_id,s.order_id,s.status,s.amount,SYSUTCDATETIME());`
	_, err := d.SQL.ExecContext(ctx, q, id, orderID, status, amount)
	return err
}

func (d *DB) UpsertInventory(ctx context.Context, sku string, delta int) error {
	// increment qty by delta, create row if not exists
	q := `MERGE dbo.inventory AS t USING (SELECT @p1 AS sku, @p2 AS delta) AS s
	ON (t.sku=s.sku)
	WHEN MATCHED THEN UPDATE SET qty=t.qty + s.delta, updated_at=SYSUTCDATETIME()
	WHEN NOT MATCHED THEN INSERT(sku,qty,updated_at) VALUES(s.sku,s.delta,SYSUTCDATETIME());`
	_, err := d.SQL.ExecContext(ctx, q, sku, delta)
	return err
}

// Queries for API

type UserWithOrders struct {
	UserID string
	Name   string
	Email  string
	Orders []Order
}

type Order struct {
	OrderID string
	UserID  string
	Amount  float64
}

type OrderWithPayment struct {
	Order
	PaymentStatus string
}

func (d *DB) GetUserWithLastOrders(ctx context.Context, id string, n int) (*UserWithOrders, error) {
	u := &UserWithOrders{Orders: []Order{}}
	row := d.SQL.QueryRowContext(ctx, "SELECT user_id,name,email FROM dbo.users WHERE user_id=@p1", id)
	if err := row.Scan(&u.UserID, &u.Name, &u.Email); err != nil {
		return nil, err
	}
	rows, err := d.SQL.QueryContext(ctx, "SELECT TOP (@p1) order_id,user_id,amount FROM dbo.orders WHERE user_id=@p2 ORDER BY updated_at DESC", n, id)
	if err != nil { return nil, err }
	defer rows.Close()
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.OrderID, &o.UserID, &o.Amount); err != nil { return nil, err }
		u.Orders = append(u.Orders, o)
	}
	return u, nil
}

func (d *DB) GetOrderWithPayment(ctx context.Context, id string) (*OrderWithPayment, error) {
	var o OrderWithPayment
	row := d.SQL.QueryRowContext(ctx, "SELECT order_id,user_id,amount FROM dbo.orders WHERE order_id=@p1", id)
	if err := row.Scan(&o.OrderID, &o.UserID, &o.Amount); err != nil { return nil, err }
	// Payment is optional; if not found, leave status empty
	row2 := d.SQL.QueryRowContext(ctx, "SELECT status FROM dbo.payments WHERE order_id=@p1", id)
	if err := row2.Scan(&o.PaymentStatus); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return &o, nil
}
