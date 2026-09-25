$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'load-env.ps1')

$provider = if ($env:LLM_PROVIDER) { $env:LLM_PROVIDER } else { 'openai' }
if ($provider -eq 'deepseek') {
    if (-not $env:DEEPSEEK_API_KEY) {
        throw 'Set DEEPSEEK_API_KEY in the environment or .env before running this test.'
    }
    if (-not $env:DEEPSEEK_MODEL) { $env:DEEPSEEK_MODEL = 'deepseek-flash' }
}
elseif ($provider -eq 'openai') {
    if (-not $env:OPENAI_API_KEY -or -not $env:OPENAI_MODEL) {
        throw 'Set OPENAI_API_KEY and OPENAI_MODEL in the environment or .env before running this test.'
    }
}
else {
    throw 'LLM_PROVIDER must be openai or deepseek.'
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
