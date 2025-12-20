-- PostgreSQL initialization script for bibd development
-- This runs automatically when the postgres container starts for the first time

-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Grant permissions (the bibd user is created by POSTGRES_USER env var)
-- Additional setup can be added here as needed

-- Log successful initialization
DO $$
BEGIN
    RAISE NOTICE 'bibd database initialized successfully';
END $$;

