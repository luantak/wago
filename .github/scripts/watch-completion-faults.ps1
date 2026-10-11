param(
  [Parameter(Mandatory)][string]$ProcDumpPath,
  [Parameter(Mandatory)][string]$DumpDirectory,
  [Parameter(Mandatory)][string]$ReadyFile,
  [Parameter(Mandatory)][string]$StopFile,
  [Parameter(Mandatory)][string]$DetachedFile
)
$ErrorActionPreference = 'Stop'
$source = 'WagoCompletionProcessStarts'
$captures = [System.Collections.Generic.List[object]]::new()
$deadline = [DateTime]::UtcNow.AddMinutes(30)
$missed = 0
$limitExceeded = $false

function Start-CaptureProcess([string[]]$Arguments) {
  $start = [System.Diagnostics.ProcessStartInfo]::new()
  $start.FileName = $ProcDumpPath
  $start.UseShellExecute = $false
  $start.RedirectStandardOutput = $true
  $start.RedirectStandardError = $true
  $start.StandardOutputEncoding = [System.Text.Encoding]::Unicode
  foreach ($argument in $Arguments) { $start.ArgumentList.Add($argument) }
  $process = [System.Diagnostics.Process]::Start($start)
  return [pscustomobject]@{
    Process = $process
    Output = $process.StandardOutput.ReadToEndAsync()
    Errors = $process.StandardError.ReadToEndAsync()
  }
}

try {
  Register-CimIndicationEvent -Query "SELECT * FROM Win32_ProcessStartTrace WHERE ProcessName = 'bash.exe'" -SourceIdentifier $source | Out-Null
  Set-Content -LiteralPath $ReadyFile -Value 'ready'
  Write-Output "Watcher ready at $([DateTime]::UtcNow.ToString('o'))"
  while (-not (Test-Path -LiteralPath $StopFile) -and [DateTime]::UtcNow -lt $deadline) {
    Wait-Event -SourceIdentifier $source -Timeout 1 | Out-Null
    foreach ($eventRecord in @(Get-Event -SourceIdentifier $source -ErrorAction SilentlyContinue)) {
      $started = $eventRecord.SourceEventArgs.NewEvent
      $targetId = [int]$started.ProcessID
      $created = [DateTime]::FromFileTimeUtc([long]$started.TIME_CREATED).ToString('o')
      Write-Output "Observed Bash PID=$targetId parent=$($started.ParentProcessID) created=$created observed=$([DateTime]::UtcNow.ToString('o'))"
      Remove-Event -EventIdentifier $eventRecord.EventIdentifier
      if ($captures.Count -ge 16) {
        $limitExceeded = $true
        Write-Output "Capture limit reached for PID=$targetId; coverage is incomplete."
        continue
      }
      try {
        $target = Get-Process -Id $targetId -ErrorAction Stop
        Write-Output "Attaching PID=$targetId image=$($target.Path) at $([DateTime]::UtcNow.ToString('o'))"
        $capture = Start-CaptureProcess -Arguments @('-accepteula', '-mm', '-e', '1', '-b', '-f', '80000004', '-n', '1', "$targetId", $DumpDirectory)
        $capture | Add-Member -NotePropertyName TargetId -NotePropertyValue $targetId
        $captures.Add($capture)
      } catch {
        $missed++
        Write-Output "Attachment unavailable for PID=$targetId at $([DateTime]::UtcNow.ToString('o')): $($_.Exception.Message)"
      }
    }
  }
  if ([DateTime]::UtcNow -ge $deadline) { throw 'Watcher exceeded its 30-minute budget.' }
} finally {
  Unregister-Event -SourceIdentifier $source -ErrorAction SilentlyContinue
  Get-Event -SourceIdentifier $source -ErrorAction SilentlyContinue | Remove-Event
  $cancellations = [System.Collections.Generic.List[object]]::new()
  foreach ($capture in $captures) {
    if (-not $capture.Process.HasExited) {
      Write-Output "Gracefully cancelling owned monitor for PID=$($capture.TargetId)"
      $cancellations.Add((Start-CaptureProcess -Arguments @('-cancel', "$($capture.TargetId)")))
    }
  }
  $cancelDeadline = [DateTime]::UtcNow.AddSeconds(5)
  foreach ($cancel in $cancellations) {
      $remaining = [Math]::Max(0, [int]($cancelDeadline - [DateTime]::UtcNow).TotalMilliseconds)
      if (-not $cancel.Process.WaitForExit($remaining)) {
        $cancel.Process.Kill()
        Write-Output 'Cancellation commands exceeded their shared five-second budget; target coverage is incomplete.'
        $missed++
      } else {
        Write-Output ($cancel.Output.GetAwaiter().GetResult())
        Write-Output ($cancel.Errors.GetAwaiter().GetResult())
      }
  }
  $stopDeadline = [DateTime]::UtcNow.AddSeconds(15)
  foreach ($capture in $captures) {
    $remaining = [Math]::Max(0, [int]($stopDeadline - [DateTime]::UtcNow).TotalMilliseconds)
    if (-not $capture.Process.WaitForExit($remaining)) {
      Write-Output "Owned monitor for PID=$($capture.TargetId) did not stop; no monitor or target was forcibly killed."
      $missed++
      continue
    }
    Write-Output "Monitor PID=$($capture.TargetId) exit=$($capture.Process.ExitCode)"
    Write-Output ($capture.Output.GetAwaiter().GetResult())
    Write-Output ($capture.Errors.GetAwaiter().GetResult())
    if ($capture.Process.ExitCode -ne 0) { $missed++ }
  }
  if (@($captures | Where-Object { -not $_.Process.HasExited }).Count -eq 0) {
    Set-Content -LiteralPath $DetachedFile -Value 'detached'
  }
  Write-Output "Capture attempts=$($captures.Count) unavailable=$missed limitExceeded=$limitExceeded"
}
if ($captures.Count -eq 0 -or $limitExceeded -or $missed -gt 0) {
  throw 'First-chance capture coverage was incomplete; an empty capture is inconclusive.'
}
