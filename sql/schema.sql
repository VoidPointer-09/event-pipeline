-- Use the consumer's Init() to create; this file serves as reference for manual creation.
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
