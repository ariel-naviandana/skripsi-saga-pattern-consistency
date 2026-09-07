# run-crash.ps1 - automates crash-in-the-middle scenarios S4/S5.
#   S4 (Orchestrator Crash, orchestration only): stops saga-orchestrator after
#      order+payment commit, while the inventory step is still in flight.
#   S5 (Kafka Down, choreography only): stops saga-kafka after the order step
#      commits, so downstream services never finish the chain.
#
# DELAY_MS=3000 (simulated slow service, proposal 3.4.1) is set so each step
# takes ~3s, creating a deterministic window to stop the component while the
# saga is still executing — otherwise the saga finishes (~100ms) faster than
# `docker stop` takes effect (~300ms) and the crash point cannot be hit.
#
# Usage: .\scripts\run-crash.ps1 -Scenario S4 -Runs 10
#        .\scripts\run-crash.ps1 -Scenario S5 -Runs 10
param(
  [string]$Scenario = "S4",
  [int]$Runs = 10
)

if ($Scenario -eq "S4") {
  $Approach = "orchestration"
  $Target = "saga-orchestrator"
  $Url = "http://localhost:8080/saga"
  $RestartDelay = 5
  $PollTable = @{ c = "saga-payment-db"; u = "payment_user"; d = "payment_db"; t = "payments" }
} elseif ($Scenario -eq "S5") {
  $Approach = "choreography"
  $Target = "saga-kafka"
  $Url = "http://localhost:8081/orders"
  $RestartDelay = 10
  $PollTable = @{ c = "saga-order-db"; u = "order_user"; d = "order_db"; t = "orders" }
} else {
  throw "scenario must be S4 or S5"
}

$outDir = "docs/runs/$Scenario/$Approach"
New-Item -ItemType Directory -Path $outDir -Force | Out-Null

$env:APPROACH = $Approach
$env:DELAY_MS = "3000"
$env:FAIL_AT_STEP = ""
$env:FAIL_AT_ATTEMPT = "0"
$env:FAIL_ON_COMPENSATE = "false"
Write-Host "Scenario: $Scenario / $Approach / $Runs runs (stop $Target mid-saga, DELAY_MS=3000)"
Write-Host "Starting services (approach=$Approach)..."
docker compose up -d order-service payment-service inventory-service shipping-service orchestrator | Out-Null
Start-Sleep -Seconds 6

function Get-FirstCol($container, $user, $db, $table, $col, $idCol) {
  $out = docker exec $container psql -U $user -d $db -t -A -c "SELECT $col FROM $table ORDER BY $idCol DESC LIMIT 1;" 2>$null
  foreach ($line in $out) { if ($line -ne "") { return $line } }
  return ""
}

function Get-Status($container, $user, $db, $table, $sagaId) {
  $out = docker exec $container psql -U $user -d $db -t -A -c "SELECT status FROM $table WHERE saga_id = '$sagaId';" 2>$null
  foreach ($line in $out) { if ($line -ne "") { return $line } }
  return ""
}

for ($i = 1; $i -le $Runs; $i++) {
  bash scripts/reset.sh *> $null

  $bodyFile = Join-Path $env:TEMP "crash-body.json"
  Set-Content -Path $bodyFile -Value '{"customer_id":"crash-1","product_id":"product-1","quantity":1,"amount":100000}' -Encoding ASCII
  $proc = Start-Process curl.exe -ArgumentList '-s','-X','POST',$Url,'-H','Content-Type: application/json','--data',("@$bodyFile") -PassThru -NoNewWindow

  $sagaId = ""
  $deadline = (Get-Date).AddSeconds(30)
  while ((Get-Date) -lt $deadline -and $sagaId -eq "") {
    $sagaId = Get-FirstCol "saga-order-db" "order_user" "order_db" "orders" "saga_id" "order_id"
    Start-Sleep -Milliseconds 50
  }

  if ($sagaId -ne "" -and $Scenario -eq "S4") {
    $payStatus = ""
    while ((Get-Date) -lt $deadline -and $payStatus -ne "committed") {
      $payStatus = Get-Status $PollTable.c $PollTable.u $PollTable.d $PollTable.t $sagaId
      Start-Sleep -Milliseconds 50
    }
  }

  if ($sagaId -eq "") {
    Write-Host "  run $i -> no saga started (timeout)"
    if (-not $proc.HasExited) { $proc.Kill() }
    docker start $Target | Out-Null
    Start-Sleep -Seconds $RestartDelay
    continue
  }

  docker stop $Target | Out-Null
  Start-Sleep -Seconds 5

  $order    = Get-Status "saga-order-db" "order_user" "order_db" "orders" $sagaId
  $payment  = Get-Status "saga-payment-db" "payment_user" "payment_db" "payments" $sagaId
  $inventory = Get-Status "saga-inventory-db" "inventory_user" "inventory_db" "inventory" $sagaId
  $shipping = Get-Status "saga-shipping-db" "shipping_user" "shipping_db" "shipments" $sagaId

  $outcome = "inconsistent"
  if ($order -eq "committed" -and $payment -eq "committed" -and $inventory -eq "committed" -and $shipping -eq "committed") {
    $outcome = "committed"
  } elseif ($order -eq "compensated") {
    $outcome = "compensated"
  } elseif ($order -eq "" -and $payment -eq "" -and $inventory -eq "" -and $shipping -eq "") {
    $outcome = "not_found"
  }

  if (-not $proc.HasExited) { $proc.Kill() }
  docker start $Target | Out-Null
  Start-Sleep -Seconds $RestartDelay

  $result = [ordered]@{
    scenario  = $Scenario
    approach  = $Approach
    run       = $i
    saga_id   = $sagaId
    order     = $order
    payment   = $payment
    inventory = $inventory
    shipping  = $shipping
    outcome   = $outcome
  }
  $out = Join-Path $outDir "run-$i.json"
  $result | ConvertTo-Json | Set-Content $out -Encoding UTF8
  Write-Host "  run $i -> $outcome (order=$order payment=$payment inventory=$inventory shipping=$shipping) -> $out"
}

Write-Host "Done. Results in $outDir"