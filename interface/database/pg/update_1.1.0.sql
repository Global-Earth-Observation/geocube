-- add index on geocube.records on datetime
CREATE INDEX idx_records_datetime ON geocube.records (datetime);
-- add jpeg support
ALTER TYPE geocube.compression ADD VALUE 'CUSTOM';
ALTER TABLE geocube.consolidation_params ADD COLUMN creation_params hstore NOT NULL default ''::hstore;