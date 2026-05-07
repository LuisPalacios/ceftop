<#
.SYNOPSIS
    Analyzes CEF/Chromium-based process trees, extracts their specific structural roles, 
    and renders a real-time, color-coded, hierarchical visualization in the console.

.DESCRIPTION
    ===========================================================================
    CHROMIUM MULTI-PROCESS ARCHITECTURE CONTEXT
    ===========================================================================
    Modern Chromium-based applications (like CEF apps, Electron, or Google Chrome itself) 
    do not run as a single monolithic process. Instead, they use a highly sandboxed, 
    multi-process architecture to isolate workloads. This improves stability (if a web 
    page crashes, it doesn't kill the whole app) and security (web code is isolated from 
    the operating system).

    Because Windows Task Manager groups these under the same executable name, debugging 
    memory or CPU spikes is difficult. This script solves that by mapping the architecture.

    Common Roles Extracted by this Script:
    - Main / Browser:   The parent process. Handles the UI, network requests, and file access.
    - Renderer:         The heavy lifters. Each tab, window, or iframe usually gets its own 
                        renderer to parse HTML/CSS and execute JavaScript securely.
    - GPU-Process:      A dedicated process that handles hardware acceleration for graphics.
    - Utility:          Sandbox helpers for specific tasks (like decoding audio, printing, etc.).
    - Crashpad-Handler: A background watcher that captures memory dumps if the app crashes.

    ===========================================================================
    SCRIPT MECHANICS
    ===========================================================================
    1. The script uses CIM (Common Information Model), the modern standard for WMI 
       (Windows Management Instrumentation), to query the OS for all running instances 
       of the target executable.
    2. It captures the 'CommandLine' property of each process, employing Regular 
       Expressions to extract the '--type=' flag passed by the Chromium engine.
    3. It builds a parent-child relationship map by linking 'ParentProcessId' to 'ProcessId'.
    4. A recursive function (Show-Node) traverses this map to draw an ASCII tree, 
       applying strict string-padding to guarantee perfectly aligned data columns.

.PARAMETER ProcessName
    The target executable to monitor. Defaults to "chrome".
    You do not need to include the ".exe" extension.

.PARAMETER Continuous
    Determines if the tool runs in a live-monitoring loop.
    $true (Default): Clears the screen and refreshes the WMI data every 2 seconds.
    $false: Prints a single, point-in-time snapshot of the process tree and exits.

.EXAMPLE
    .\Get-ChromiumTree.ps1
    Launches the live monitor for the default "chrome" processes.

.EXAMPLE
    .\Get-ChromiumTree.ps1 -ProcessName "chrome" -Continuous $false
    Generates a single snapshot of Google Chrome's process tree and exits immediately.
#>

param (
    [Parameter(Mandatory=$false, HelpMessage="Target application executable name")]
    [string]$ProcessName = "chrome",

    [Parameter(Mandatory=$false, HelpMessage="Enable live refresh loop")]
    [bool]$Continuous = $true
)

# =============================================================================
# ENVIRONMENT & CONFIGURATION
# =============================================================================

# Define the console colors for each specific Chromium architectural role.
# This visual dictionary makes it easy to spot resource hogs at a glance.
$RoleColors = @{
    "Main / Browser"   = "Green"
    "renderer"         = "DarkYellow"
    "gpu-process"      = "Magenta"
    "utility"          = "DarkCyan"
    "crashpad-handler" = "Red"
    "watcher"          = "DarkRed"
    "plugin"           = "Cyan"
    "ppapi"            = "Cyan"
    "ppapi-broker"     = "Cyan"
    "extension"        = "Yellow"
    "zygote"           = "DarkMagenta" # Mainly Linux, included for full CEF compatibility
    "Default"          = "Gray"        # Fallback for undocumented flags
}

# Standardize the input string to ensure precise WMI querying.
# This regex removes ".exe" if the user typed it, then strictly appends it.
$exeName = ($ProcessName -replace '(?i)\.exe$', '') + '.exe'

# =============================================================================
# RECURSIVE TREE RENDERING FUNCTION
# =============================================================================

function Show-Node {
    param (
        [int]$ParentId,
        [string]$Prefix 
    )

    # Fetch children of the current node, sorted by heaviest memory usage first
    $children = $processList | Where-Object { $_.PPID -eq $ParentId } | Sort-Object MemMB -Descending
    $count = $children.Count
    $i = 0

    foreach ($child in $children) {
        $i++
        # Determine if this is the final child branch to close the ASCII visual line
        $isLast = ($i -eq $count)
        $branch = if ($isLast) { "\_ " } else { "|_ " }
        
        # ---------------------------------------------------------------------
        # COLUMN ALIGNMENT LOGIC (CHILDREN)
        # ---------------------------------------------------------------------
        # 1. Pad the PID string to 7 chars (e.g., "[3560] " vs "[36544]") 
        #    so the role name always starts at the exact same horizontal position.
        $pidRaw = "[{0}]" -f $child.PID
        $pidPadded = "{0,-7}" -f $pidRaw

        # 2. Pad the Chromium Role to 16 characters (the length of "crashpad-handler").
        #    This guarantees the "Threads:" text starts exactly 2 spaces after the longest role.
        $rolePadded = "{0,-16}" -f $child.Role

        # Assemble the precisely formatted string
        $line = "{0}{1}{2} {3}  Threads: {4,4}   Mem: {5,8} MB" -f $Prefix, $branch, $pidPadded, $rolePadded, $child.Threads, $child.MemMB
        
        # Apply role-based color highlighting
        $nodeColor = $RoleColors["Default"]
        if ($RoleColors.ContainsKey($child.Role)) {
            $nodeColor = $RoleColors[$child.Role]
        }
        
        Write-Host $line -ForegroundColor $nodeColor

        # Calculate the vertical line tracking for the next recursive depth
        if ($isLast) {
            $nextPrefix = $Prefix + "   "  # Blank space if parent branch is closed
        } else {
            $nextPrefix = $Prefix + "|  "  # Continuing line if parent has more siblings
        }
        
        # Traverse deeper into the tree
        Show-Node -ParentId $child.PID -Prefix $nextPrefix
    }
}

# =============================================================================
# MAIN EXECUTION THREAD (LIVE LOOP)
# =============================================================================

while ($true) {

    # Fetch OS-level process data via CIM
    $processes = Get-CimInstance Win32_Process -Filter "Name = '$exeName'"

    # Refresh the console buffer if live monitoring is enabled
    if ($Continuous) { Clear-Host }

    # Handle inactive states gracefully without crashing the loop
    if ($processes.Count -eq 0) {
        Write-Warning "No running processes found for '$exeName'."
        if (-not $Continuous) { break }
        Start-Sleep -Seconds 2
        continue
    }

    # Transform raw WMI objects into a streamlined dataset.
    # Saved to $script: scope so the Show-Node function can access it globally.
    $script:processList = foreach ($proc in $processes) {
        $cmdLine = $proc.CommandLine
        
        # Baseline assumption: The parent process launching the app lacks a --type flag
        $role = "Main / Browser" 
        
        # Parse the execution arguments. If it finds --type=XXXX, extract XXXX.
        if (![string]::IsNullOrWhiteSpace($cmdLine) -and $cmdLine -match "--type=([a-zA-Z0-9\-]+)") {
            $role = $matches[1]
        }

        [PSCustomObject]@{
            PID     = $proc.ProcessId
            PPID    = $proc.ParentProcessId
            Role    = $role
            Threads = $proc.ThreadCount
            MemMB   = [math]::Round($proc.WorkingSetSize / 1MB, 2)
        }
    }

    # -------------------------------------------------------------------------
    # RENDER HEADER
    # -------------------------------------------------------------------------
    $timeStamp = Get-Date -Format "HH:mm:ss"
    Write-Host "`n$exeName" -ForegroundColor Cyan -NoNewline
    Write-Host " (Last Updated: $timeStamp)" -ForegroundColor DarkGray

    # Identify absolute root processes (The entry points of the application)
    $allPids = $processList.PID
    $roots = $processList | Where-Object { $_.PPID -notin $allPids }

    # -------------------------------------------------------------------------
    # RENDER ROOTS & TRIGGER TREE
    # -------------------------------------------------------------------------
    foreach ($root in $roots) {
        
        $rootColor = $RoleColors["Default"]
        if ($RoleColors.ContainsKey($root.Role)) {
            $rootColor = $RoleColors[$root.Role]
        }

        # As requested, the Root node retains natural spacing without heavy role padding
        $pidRaw = "[{0}]" -f $root.PID
        $pidPadded = "{0,-7}" -f $pidRaw
        $rolePadded = "{0,-16}" -f $root.Role
        $line = "\_ {0} {1}    Threads: {2,4}   Mem: {3,8} MB" -f $pidPadded, $rolePadded, $root.Threads, $root.MemMB
        
        Write-Host $line -ForegroundColor $rootColor
        
        # Begin drawing children recursively under this root, indented by 2 spaces
        Show-Node -ParentId $root.PID -Prefix "  "
    }

    Write-Host "" 
    
    # Exit condition for single-run mode
    if (-not $Continuous) {
        break
    }

    # Throttle the query to prevent excessive CPU polling on the host machine
    Start-Sleep -Seconds 2
}
