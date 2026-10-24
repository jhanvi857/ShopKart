<#
.SYNOPSIS
    ShopKart Operational Script for Windows PowerShell
.DESCRIPTION
    Runs infrastructure in Docker (no heavy builds), and runs Go backend + Vite frontend natively.
#>

param (
    [Parameter(Position = 0, Mandatory = $false)]
    [string]$Command = "help"
)

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $ScriptDir

function Show-Help {
    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host "  ShopKart Lightweight Native Runner                      " -ForegroundColor Cyan
    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host "Usage: .\run.ps1 <command>`n"
    Write-Host "Core Commands:"
    Write-Host "  infra              - Start pre-built Postgres/Redis/Kafka/Prometheus/Grafana (Takes 3s, zero builds)" -ForegroundColor Green
    Write-Host "  backend            - Run all 10 Go services natively using your system's Go" -ForegroundColor Green
    Write-Host "  storefront         - Run React + Vite storefront natively (http://localhost:3000)" -ForegroundColor Green
    Write-Host "  up                 - Start infra + backend"
    Write-Host "  down               - Stop Docker infrastructure containers"
    Write-Host "  test               - Run Go unit tests"
    Write-Host "`nLoad & Fault Testing Commands:"
    Write-Host "  load-baseline      - Run baseline k6 load test"
    Write-Host "  load-spike         - Run checkout spike k6 test"
    Write-Host "  fault-db           - Inject Postgres inventory row-lock"
    Write-Host "  revert-db          - Release Postgres row-lock"
    Write-Host "  fault-cache        - Stop Redis cache"
    Write-Host "  revert-cache       - Restart Redis cache"
    Write-Host "  fault-slow-query   - Saturate DB connection pool"
    Write-Host "  revert-slow-query  - Terminate slow queries"
    Write-Host "  fault-memory       - Trigger memory leak in payments"
    Write-Host "  revert-memory      - Clear memory leak"
    Write-Host "  fault-netem        - Inject network latency into bank mock"
    Write-Host "  revert-netem       - Revert network latency"
    Write-Host "  revert-all         - Restore clean healthy baseline"
}

switch ($Command.ToLower()) {
    "init" {
        if (-not (Test-Path ".env")) {
            Copy-Item ".env.example" ".env"
            Write-Host "[OK] Created .env from .env.example" -ForegroundColor Green
        } else {
            Write-Host "[INFO] .env already exists" -ForegroundColor Yellow
        }
    }
    "infra" {
        & $MyInvocation.MyCommand.Definition init
        Write-Host "Starting prebuilt infrastructure datastores (PostgreSQL, Redis, Kafka, Prometheus, Grafana, Jaeger)..." -ForegroundColor Cyan
        docker compose up -d
        Write-Host "[OK] Infrastructure datastores are online!" -ForegroundColor Green
    }
    "backend" {
        Write-Host "Starting all 10 Go backend microservices on host (ports 9000-9009)..." -ForegroundColor Cyan
        Push-Location "services"
        go run main.go
        Pop-Location
    }
    "storefront" {
        Write-Host "Starting React + Vite storefront on http://localhost:3000..." -ForegroundColor Cyan
        Push-Location "storefront"
        if (-not (Test-Path "node_modules")) {
            Write-Host "Installing storefront dependencies locally..." -ForegroundColor Yellow
            npm install
        }
        npm run dev
        Pop-Location
    }
    "up" {
        & $MyInvocation.MyCommand.Definition infra
        Write-Host "`nTo start the Go backend and Storefront:" -ForegroundColor Yellow
        Write-Host "  Terminal 1: .\run.ps1 backend" -ForegroundColor White
        Write-Host "  Terminal 2: .\run.ps1 storefront" -ForegroundColor White
    }
    "down" {
        docker compose down
    }
    "test" {
        Push-Location "services"
        go test -v ./...
        Pop-Location
    }
    "load-baseline" {
        k6 run load/baseline.js
    }
    "load-spike" {
        k6 run load/spike_checkout.js
    }
    "load-cache" {
        k6 run load/flash_sale_catalog.js
    }
    "load-queue" {
        k6 run load/queue_flood.js
    }
    "load-mixed" {
        k6 run load/mixed_realistic.js
    }
    "fault-crash" {
        Write-Host "[FAULT] Terminating inventory service on port 9004..." -ForegroundColor Yellow
        Get-NetTCPConnection -LocalPort 9004 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force }
    }
    "fault-cache" {
        docker stop shopkart-redis
    }
    "revert-cache" {
        docker start shopkart-redis
    }
    "fault-db" {
        docker exec -d shopkart-postgres psql -U shopkart_user -d shopkart_db -c "BEGIN; SELECT * FROM inventory WHERE product_id = 'el_01' FOR UPDATE; SELECT pg_sleep(300); COMMIT;"
        Write-Host "[FAULT] Row lock acquired on el_01 for 300s." -ForegroundColor Yellow
    }
    "revert-db" {
        docker exec shopkart-postgres psql -U shopkart_user -d shopkart_db -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE '%pg_sleep%' AND pid <> pg_backend_pid();"
        Write-Host "[REVERT] Blocking transactions terminated." -ForegroundColor Green
    }
    "fault-slow-query" {
        for ($i=1; $i -le 12; $i++) {
            docker exec -d shopkart-postgres psql -U shopkart_user -d shopkart_db -c "SELECT pg_sleep(120), count(*) FROM generate_series(1, 5000000);"
        }
        Write-Host "[FAULT] 12 slow queries running to saturate connection pool." -ForegroundColor Yellow
    }
    "revert-slow-query" {
        docker exec shopkart-postgres psql -U shopkart_user -d shopkart_db -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'active' AND query LIKE '%generate_series%' AND pid <> pg_backend_pid();"
        Write-Host "[REVERT] Slow queries terminated." -ForegroundColor Green
    }
    "fault-memory" {
        Invoke-RestMethod -Uri "http://localhost:9005/admin/fault/leak-enable" -Method Post -Headers @{"X-Admin-Key" = "shopkart-ops-secret-key-prod-998"}
        for ($i=1; $i -le 20; $i++) {
            $body = @{
                order_id = "ord_leak_$i"
                amount = 10.0
                currency = "INR"
                idempotency_key = "ik_leak_$i"
                card_token = "tok_leak"
            } | ConvertTo-Json
            Invoke-RestMethod -Uri "http://localhost:9005/payments/charge" -Method Post -ContentType "application/json" -Body $body | Out-Null
        }
        Write-Host "[FAULT] Memory leak triggered in payments service." -ForegroundColor Yellow
    }
    "revert-memory" {
        Invoke-RestMethod -Uri "http://localhost:9005/admin/fault/leak-disable" -Method Post -Headers @{"X-Admin-Key" = "shopkart-ops-secret-key-prod-998"} | Out-Null
        Invoke-RestMethod -Uri "http://localhost:9005/admin/fault/leak-clear" -Method Post -Headers @{"X-Admin-Key" = "shopkart-ops-secret-key-prod-998"} | Out-Null
        Write-Host "[REVERT] Memory leak neutralized." -ForegroundColor Green
    }
    "fault-netem" {
        $body = @{ base_latency_ms = 350; jitter_ms = 75 } | ConvertTo-Json
        Invoke-RestMethod -Uri "http://localhost:9008/admin/config" -Method Post -ContentType "application/json" -Body $body | Out-Null
        Write-Host "[FAULT] Injected 350ms link latency into bank mock." -ForegroundColor Yellow
    }
    "revert-netem" {
        $body = @{ base_latency_ms = 35; jitter_ms = 15 } | ConvertTo-Json
        Invoke-RestMethod -Uri "http://localhost:9008/admin/config" -Method Post -ContentType "application/json" -Body $body | Out-Null
        Write-Host "[REVERT] Network delay restored to baseline (35ms)." -ForegroundColor Green
    }
    "revert-all" {
        & $MyInvocation.MyCommand.Definition revert-cache
        & $MyInvocation.MyCommand.Definition revert-db
        & $MyInvocation.MyCommand.Definition revert-slow-query
        & $MyInvocation.MyCommand.Definition revert-memory
        & $MyInvocation.MyCommand.Definition revert-netem
        Write-Host "[REVERT] All faults reverted." -ForegroundColor Green
    }
    default {
        Show-Help
    }
}
