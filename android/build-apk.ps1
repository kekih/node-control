$ErrorActionPreference = "Stop"

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "     Сборка Android APK для приложения Node       " -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$AdbDir = "D:\Desktop\ADB"
$BuildTools = "$AdbDir\build-tools\34.0.0"
$Aapt2 = "$BuildTools\aapt2.exe"
$Aapt = "$BuildTools\aapt.exe"
$D8 = "$BuildTools\d8.bat"
$Zipalign = "$BuildTools\zipalign.exe"
$Apksigner = "$BuildTools\apksigner.bat"
$AndroidJar = "$AdbDir\platforms\android-34\android.jar"
$Keystore = "$AdbDir\Project\debug.keystore"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

$OutDir = Resolve-Path "$ScriptDir\..\bin"
$BuildDir = "$ScriptDir\build"

if (Test-Path $BuildDir) {
    Remove-Item -Recurse -Force $BuildDir
}
New-Item -ItemType Directory -Path "$BuildDir\gen" -Force | Out-Null
New-Item -ItemType Directory -Path "$BuildDir\obj" -Force | Out-Null
New-Item -ItemType Directory -Path "$BuildDir\dex" -Force | Out-Null
New-Item -ItemType Directory -Path "$ScriptDir\assets" -Force | Out-Null

Write-Host "[1/6] Синхронизация веб-ассетов..." -ForegroundColor Yellow
Copy-Item -Path "$ScriptDir\..\internal\web\static\*" -Destination "$ScriptDir\assets" -Recurse -Force

Write-Host "[2/6] Компиляция ресурсов (aapt2 compile)..." -ForegroundColor Yellow
& $Aapt2 compile --dir "$ScriptDir\res" -o "$BuildDir\compiled_res.zip"
if ($LASTEXITCODE -ne 0) { throw "aapt2 compile failed" }

Write-Host "[3/6] Линковка APK и генерация R.java (aapt2 link)..." -ForegroundColor Yellow
& $Aapt2 link -I $AndroidJar --manifest "$ScriptDir\AndroidManifest.xml" --min-sdk-version 24 --target-sdk-version 34 --version-code 1 --version-name "1.0.0" "$BuildDir\compiled_res.zip" -A "$ScriptDir\assets" --java "$BuildDir\gen" -o "$BuildDir\base.apk"
if ($LASTEXITCODE -ne 0) { throw "aapt2 link failed" }

Write-Host "[4/6] Компиляция Java кода (javac)..." -ForegroundColor Yellow
$JavaSources = @(
    "$ScriptDir\src\com\node\control\MainActivity.java",
    "$BuildDir\gen\com\node\control\R.java"
)
& javac --release 8 -cp $AndroidJar -d "$BuildDir\obj" $JavaSources
if ($LASTEXITCODE -ne 0) { throw "javac compilation failed" }

Write-Host "[5/6] Создание Dalvik Executable (d8)..." -ForegroundColor Yellow
$ClassFiles = Get-ChildItem -Path "$BuildDir\obj\com\node\control" -Filter "*.class" | ForEach-Object { $_.FullName }
& cmd /c "$D8 --output `"$BuildDir\dex`" --lib `"$AndroidJar`" $($ClassFiles -join ' ')"
if ($LASTEXITCODE -ne 0) { throw "d8 dexing failed" }

Write-Host "[6/6] Упаковка, выравнивание и подпись APK..." -ForegroundColor Yellow
Set-Location $BuildDir
Copy-Item -Path "$BuildDir\dex\classes.dex" -Destination "$BuildDir\classes.dex" -Force
& $Aapt add base.apk classes.dex
& $Zipalign -f -p 4 base.apk aligned.apk

$FinalApk = "$OutDir\Node.apk"
& cmd /c "`"$Apksigner`" sign --ks `"$Keystore`" --ks-pass pass:android --out `"$FinalApk`" aligned.apk"
if ($LASTEXITCODE -ne 0) { throw "apksigner failed" }

Set-Location $ScriptDir

if (Test-Path $FinalApk) {
    $apkSize = (Get-Item $FinalApk).Length / 1MB
    Write-Host "==================================================" -ForegroundColor Green
    Write-Host "[✓] APK успешно собран и подписан!" -ForegroundColor Green
    Write-Host "Файл: $FinalApk ($([math]::Round($apkSize, 2)) MB)" -ForegroundColor Green
    Write-Host "==================================================" -ForegroundColor Green
}
