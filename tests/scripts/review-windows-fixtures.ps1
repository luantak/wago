param(
    [Parameter(Mandatory)][string]$SourceRoot,
    [Parameter(Mandatory)][string]$EvidenceRoot,
    [Parameter(Mandatory)][string]$ExpectedSource
)

$ErrorActionPreference = 'Stop'
$source = (Resolve-Path $SourceRoot).Path
$evidence = (New-Item -ItemType Directory -Force $EvidenceRoot).FullName
$sourceSha = (& git -C $source rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $sourceSha -ne $ExpectedSource) { throw 'Unexpected source revision' }

# This workflow runs only the repaired fixtures already in the PR. It does not
# restore old faulting fixtures or establish the cause of the historical crash.
Add-Type @'
using System.Runtime.InteropServices;
public static class ReviewHostFeatures {
    [DllImport("kernel32.dll")]
    public static extern ulong GetEnabledXStateFeatures();
}
'@
$xstate = [ReviewHostFeatures]::GetEnabledXStateFeatures()
$identity = [ordered]@{
    source_sha = $sourceSha
    workflow_sha = $env:GITHUB_SHA
    cpu = @(Get-CimInstance Win32_Processor | Select-Object Name, Manufacturer, Architecture)
    os = Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, OSArchitecture
    xstate_mask_hex = ('0x{0:x}' -f $xstate)
    amx_xtiledata_enabled = (($xstate -band ([uint64]1 -shl 18)) -ne 0)
    gc_percent = $env:GOGC
    gomaxprocs = $env:GOMAXPROCS
    powershell = $PSVersionTable.PSVersion.ToString()
    qualification = 'Current repaired fixtures only; no historical causal proof'
}
$identity | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $evidence 'host.json')

$fixtureBlobs = [ordered]@{
    'src/core/compiler/backend/railshot/amd64/internal_error_test.go' = '2ea2d67db11cf7927f5a52a05f1490f1bff4e316'
    'src/core/compiler/backend/railshot/amd64/source_context_enabled_test.go' = 'a5ece2d03ab16db2da1f644e1b3cf5758b83d4ba'
}
foreach ($file in $fixtureBlobs.Keys) {
    $blob = (& git -C $source rev-parse "HEAD:$file").Trim()
    if ($LASTEXITCODE -ne 0 -or $blob -ne $fixtureBlobs[$file]) { throw "Unexpected repaired fixture: $file" }
}
$fixtureBlobs | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $evidence 'fixture-blobs.json')

function Invoke-Recorded {
    param([string]$Label, [string]$Executable, [string[]]$Arguments)
    [ordered]@{ executable = $Executable; arguments = $Arguments } |
        ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 (Join-Path $evidence "$Label.command.json")
    $info = [System.Diagnostics.ProcessStartInfo]::new($Executable)
    $info.WorkingDirectory = $source
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    foreach ($argument in $Arguments) { $info.ArgumentList.Add($argument) }
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $info
    $outFile = [System.IO.File]::Create((Join-Path $evidence "$Label.stdout"))
    $errFile = [System.IO.File]::Create((Join-Path $evidence "$Label.stderr"))
    try {
        if (-not $process.Start()) { throw "Cannot start $Label" }
        $outCopy = $process.StandardOutput.BaseStream.CopyToAsync($outFile)
        $errCopy = $process.StandardError.BaseStream.CopyToAsync($errFile)
        $process.WaitForExit()
        $null = $outCopy.GetAwaiter().GetResult()
        $null = $errCopy.GetAwaiter().GetResult()
        $exitCode = $process.ExitCode
    } finally {
        $outFile.Dispose()
        $errFile.Dispose()
        $process.Dispose()
    }
    $exitCode | Set-Content -Encoding ascii (Join-Path $evidence "$Label.exit")
    if ($exitCode -ne 0) {
        Get-Content (Join-Path $evidence "$Label.stdout")
        Get-Content (Join-Path $evidence "$Label.stderr")
        throw "$Label failed with exit $exitCode"
    }
}

$go = (Get-Command go).Source
Invoke-Recorded 'go-version' $go @('version')
Invoke-Recorded 'go-platform' $go @('env', 'GOOS', 'GOARCH', 'GOHOSTOS', 'GOHOSTARCH')
$platform = @(Get-Content (Join-Path $evidence 'go-platform.stdout'))
if (($platform -join ',') -ne 'windows,amd64,windows,amd64') { throw 'Native Windows AMD64 is required' }
$binary = Join-Path $evidence 'backend.test.exe'
Invoke-Recorded 'build' $go @('test', '-tags=wago_regalloccheck', '-c', '-o', $binary, './src/core/compiler/backend/railshot/amd64')
Get-FileHash -Algorithm SHA256 $binary | Select-Object Path, Hash |
    ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $evidence 'binary-sha256.json')

$names = @(
    'TestCompilePanicHasInternalClassification',
    'TestCompileDebugPanicRethrowsOriginal',
    'TestSourceContextSurvivesEarlyFunctionStateFailure',
    'TestMemoryCopyPatchScratchDoesNotAllocate'
)
$pattern = '^(' + ($names -join '|') + ')$'
Invoke-Recorded 'focused' $binary @('-test.v', "-test.run=$pattern", '-test.count=1000', '-test.timeout=10m')
$focused = Get-Content -Raw (Join-Path $evidence 'focused.stdout')
foreach ($name in $names) {
    $passes = [regex]::Matches($focused, ('(?m)^--- PASS: ' + [regex]::Escape($name) + ' \(')).Count
    if ($passes -ne 1000) { throw "Expected 1000 passes for $name; got $passes" }
}
Invoke-Recorded 'checked-backend' $binary @(
    '-test.count=10',
    '-test.skip=^TestSharedEstablishedPathPairs/independent-v8$',
    '-test.timeout=12m'
)
[ordered]@{
    result = 'pass'
    source_sha = $sourceSha
    repaired_fixture_passes = 4000
    checked_backend_runs = 10
    independent_v8 = 'covered once by normal PR CI; excluded from repeated fixture stress'
    amx_xtiledata_enabled = $identity.amx_xtiledata_enabled
    qualification = $identity.qualification
} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $evidence 'summary.json')
