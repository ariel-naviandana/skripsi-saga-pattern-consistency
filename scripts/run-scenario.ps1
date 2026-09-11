# run-scenario.ps1 - runs a scenario for N iterations and saves JSON per run.
# Usage: .\scripts\run-scenario.ps1 -Scenario S1 -Approach choreography -Runs 30
param(
  [string]$Scenario = "S1",
  [string]$Approach = "choreography",
  [int]$Runs = 30
)

if ($Approach -notin @("choreography", "orchestration")) {
  throw "approach must be choreography|orchestration"
}

$outDir = "docs/runs/$Scenario/$Approach"
New-Item -ItemType Directory -Path $outDir -Force | Out-Null

switch ($Scenario) {
  { $_ -in @("S1", "S7") } { $env:FAIL_AT_STEP = ""; $env:FAIL_AT_ATTEMPT = "0"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "false" }
  "S2" { $env:FAIL_AT_STEP = "shipping"; $env:FAIL_AT_ATTEMPT = "1"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "false" }
  "S3" { $env:FAIL_AT_STEP = "inventory"; $env:FAIL_AT_ATTEMPT = "1"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "false" }
  "S6" { $env:FAIL_AT_STEP = "inventory"; $env:FAIL_AT_ATTEMPT = "1"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "true"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "false" }
  "S8" { $env:FAIL_AT_STEP = ""; $env:FAIL_AT_ATTEMPT = "0"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = "saga.order.created"; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "false" }
  "S9" { $env:FAIL_AT_STEP = ""; $env:FAIL_AT_ATTEMPT = "0"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = "inventory"; $env:SELECTIVE_COMPENSATE = "false" }
  "S9s" { $env:FAIL_AT_STEP = ""; $env:FAIL_AT_ATTEMPT = "0"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = "inventory"; $env:SELECTIVE_COMPENSATE = "true" }
  "S2s" { $env:FAIL_AT_STEP = "shipping"; $env:FAIL_AT_ATTEMPT = "1"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "true" }
  "S3s" { $env:FAIL_AT_STEP = "inventory"; $env:FAIL_AT_ATTEMPT = "1"; $env:DELAY_MS = "0"; $env:FAIL_ON_COMPENSATE = "false"; $env:DROP_EVENT = ""; $env:DROP_RESPONSE_AT_STEP = ""; $env:SELECTIVE_COMPENSATE = "true" }
  default { throw "unsupported scenario $Scenario" }
}

$count = if ($Scenario -eq "S7") { 500 } else { 1 }

Write-Host "Scenario: $Scenario / $Approach / $Runs runs"
Write-Host "Fault: FAIL_AT_STEP=$env:FAIL_AT_STEP FAIL_AT_ATTEMPT=$env:FAIL_AT_ATTEMPT FAIL_ON_COMPENSATE=$env:FAIL_ON_COMPENSATE DROP_EVENT=$env:DROP_EVENT DROP_RESPONSE_AT_STEP=$env:DROP_RESPONSE_AT_STEP SELECTIVE_COMPENSATE=$env:SELECTIVE_COMPENSATE"

$env:APPROACH = $Approach

Write-Host "Starting services (approach=$Approach)..."
docker compose up -d order-service payment-service inventory-service shipping-service orchestrator | Out-Null
Start-Sleep -Seconds 6

for ($i = 1; $i -le $Runs; $i++) {
  bash scripts/reset.sh *> $null
  $out = Join-Path $outDir "run-$i.json"
  go run ./cmd/workload-generator -approach $Approach -count $count -out $out | Out-Null
  if ($LASTEXITCODE -ne 0) { Write-Host "  run $i failed" }
  Write-Host "  run $i done -> $out"
}

Write-Host "Done. Results in $outDir"