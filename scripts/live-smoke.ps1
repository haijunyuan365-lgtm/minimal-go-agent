$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'load-env.ps1')

$env:AGENT_CONFIG = if ($env:AGENT_CONFIG) { $env:AGENT_CONFIG } else { Join-Path $projectRoot 'config.yml' }
$env:AGENT_LIVE_TEST = '1'
Push-Location $projectRoot
try {
    go test ./internal/llm ./internal/agent -run '^TestLive' -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}
