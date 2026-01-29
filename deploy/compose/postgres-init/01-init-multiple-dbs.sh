#!/usr/bin/env bash
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=auth_db_password="$AUTH_DB_PASSWORD" \
  --set=business_db_password="$BUSINESS_DB_PASSWORD" \
  --set=booking_db_password="$BOOKING_DB_PASSWORD" \
  --set=billing_db_password="$BILLING_DB_PASSWORD" \
  --set=scheduler_db_password="$SCHEDULER_DB_PASSWORD" \
  --set=notification_db_password="$NOTIFICATION_DB_PASSWORD" \
  --set=analytics_db_password="$ANALYTICS_DB_PASSWORD" <<'EOSQL'
SELECT format('CREATE ROLE auth_user LOGIN PASSWORD %L', :'auth_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_user') \gexec
SELECT format('CREATE ROLE business_user LOGIN PASSWORD %L', :'business_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'business_user') \gexec
SELECT format('CREATE ROLE booking_user LOGIN PASSWORD %L', :'booking_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'booking_user') \gexec
SELECT format('CREATE ROLE billing_user LOGIN PASSWORD %L', :'billing_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'billing_user') \gexec
SELECT format('CREATE ROLE scheduler_user LOGIN PASSWORD %L', :'scheduler_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'scheduler_user') \gexec
SELECT format('CREATE ROLE notification_user LOGIN PASSWORD %L', :'notification_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'notification_user') \gexec
SELECT format('CREATE ROLE analytics_user LOGIN PASSWORD %L', :'analytics_db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'analytics_user') \gexec

SELECT 'CREATE DATABASE auth_db OWNER auth_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'auth_db') \gexec
SELECT 'CREATE DATABASE business_db OWNER business_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'business_db') \gexec
SELECT 'CREATE DATABASE booking_db OWNER booking_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'booking_db') \gexec
SELECT 'CREATE DATABASE billing_db OWNER billing_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'billing_db') \gexec
SELECT 'CREATE DATABASE scheduler_db OWNER scheduler_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'scheduler_db') \gexec
SELECT 'CREATE DATABASE notification_db OWNER notification_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'notification_db') \gexec
SELECT 'CREATE DATABASE analytics_db OWNER analytics_user'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'analytics_db') \gexec
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "booking_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS btree_gist;
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "auth_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "notification_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "scheduler_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "business_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "billing_db" <<'EOSQL'
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
EOSQL
