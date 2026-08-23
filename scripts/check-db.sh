#!/bin/bash
# Script untuk memeriksa status database synchronization

echo "======================================"
echo "Database Synchronization Check"
echo "======================================"
echo ""

# Check if containers are running
echo "1. Checking Docker containers..."
docker ps --filter "name=hawking" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""

# Check migration version
echo "2. Checking migration version..."
docker exec hawking-postgres psql -U postgres -d hawking -c "SELECT version, dirty FROM schema_migrations;" 2>/dev/null || echo "❌ Migration table not found or containers not running"
echo ""

# Check if tables exist
echo "3. Checking database tables..."
docker exec hawking-postgres psql -U postgres -d hawking -c "\dt" 2>/dev/null || echo "❌ Cannot connect to database"
echo ""

# Check jadwal data
echo "4. Checking jadwal_kelas records..."
docker exec hawking-postgres psql -U postgres -d hawking -c "SELECT COUNT(*) as total_jadwal FROM jadwal_kelas;" 2>/dev/null || echo "❌ Table jadwal_kelas not found"
echo ""

echo "======================================"
echo "Check completed!"
echo "======================================"
