$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$envFile = Join-Path $projectRoot '.env'

if (Test-Path -LiteralPath $envFile) {
    foreach ($line in Get-Content -LiteralPath $envFile) {
        if ($line -match '^\s*(AGENT_CONFIG|OPENAI_API_KEY|DEEPSEEK_API_KEY)\s*=\s*(.*?)\s*$') {
            $name = $Matches[1]
            $value = $Matches[2].Trim().Trim('"').Trim("'")
            [Environment]::SetEnvironmentVariable($name, $value, 'Process')
        }
    }
}

if ($env:AGENT_CONFIG -and -not [System.IO.Path]::IsPathRooted($env:AGENT_CONFIG)) {
    $env:AGENT_CONFIG = Join-Path $projectRoot $env:AGENT_CONFIG
}
