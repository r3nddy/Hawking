#!/bin/bash
# Script untuk reset database dan re-run migrations

echo "======================================"
echo "Database Reset & Re-migration"
echo "======================================"
echo ""

echo "⚠️  This will DELETE all data in the database!"
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Operation cancelled."
    exit 0
fi

echo ""
echo "1. Stopping containers..."
docker compose down

echo ""
echo "2. Removing postgres volume..."
docker volume rm hawking_postgres_data 2>/dev/null || echo "Volume already removed or doesn't exist"

echo ""
echo "3. Starting containers..."
docker compose up -d

echo ""
echo "4. Waiting for database to be ready..."
sleep 5

echo ""
echo "5. Checking migration status..."
docker exec hawking-postgres psql -U postgres -d hawking -c "SELECT version, dirty FROM schema_migrations;"

echo ""
echo "6. Checking tables..."
docker exec hawking-postgres psql -U postgres -d hawking -c "\dt"

echo ""
echo "7. Verifying jadwal data..."
docker exec hawking-postgres psql -U postgres -d hawking -c "SELECT COUNT(*) FROM jadwal_kelas;"

echo ""
echo "======================================"
echo "Reset completed!"
echo "======================================"
