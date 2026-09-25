$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'load-env.ps1')

if (-not $env:OPENAI_API_KEY -or -not $env:OPENAI_MODEL) {
    throw 'Set OPENAI_API_KEY and OPENAI_MODEL in the environment or .env before running this test.'
}

$env:AGENT_LIVE_TEST = '1'
Push-Location $projectRoot
try {
    go test ./internal/llm ./internal/agent -run '^TestLive' -v -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}
