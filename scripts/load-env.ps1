$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$envFile = Join-Path $projectRoot '.env'

if (Test-Path -LiteralPath $envFile) {
    foreach ($line in Get-Content -LiteralPath $envFile) {
        if ($line -match '^\s*(LLM_PROVIDER|OPENAI_API_KEY|OPENAI_MODEL|OPENAI_REASONING_SUMMARY|DEEPSEEK_API_KEY|DEEPSEEK_MODEL|DEEPSEEK_ENDPOINT|DEEPSEEK_REASONING_EFFORT)\s*=\s*(.*?)\s*$') {
            $name = $Matches[1]
            $value = $Matches[2].Trim().Trim('"').Trim("'")
            [Environment]::SetEnvironmentVariable($name, $value, 'Process')
        }
    }
}
