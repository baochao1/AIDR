$ErrorActionPreference = 'Stop'
function Check { if ($LASTEXITCODE -ne 0) { throw "Command exited $LASTEXITCODE" } }
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    $suffix = if ($IsWindows -or $env:OS -eq 'Windows_NT') { '.exe' } else { '' }
    New-Item -ItemType Directory -Force build | Out-Null
    go test ./...
    Check
    go vet ./...
    Check
    go build -o "build/supervisor-demo$suffix" ./cmd/supervisor-demo
    Check
    go build -o "build/mock-go$suffix" ./cmd/mock-go
    Check
    go build -o "build/ipc-poc$suffix" ./cmd/ipc-poc
    Check
    & "./build/ipc-poc$suffix" "./build/mock-go$suffix"
    Check
    python -m pip install -r plugins/mock-python/requirements.txt
    Check
    & "./build/ipc-poc$suffix" python plugins/mock-python/mock.py
    Check
    python -m PyInstaller --noconfirm --onefile --distpath build --workpath build/pyinstaller --specpath build --name mock-python plugins/mock-python/mock.py
    Check
    python scripts/e2e.py "build/supervisor-demo$suffix" "build/mock-go$suffix" "build/mock-python$suffix"
    Check
} finally {
    Pop-Location
}
