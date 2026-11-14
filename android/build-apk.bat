@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

echo ==================================================
echo         Сборка Android APK для приложения Node     
echo ==================================================

set "ADB_DIR=D:\Desktop\ADB"
set "BUILD_TOOLS=%ADB_DIR%\build-tools\34.0.0"
set "AAPT2=%BUILD_TOOLS%\aapt2.exe"
set "AAPT=%BUILD_TOOLS%\aapt.exe"
set "D8=%BUILD_TOOLS%\d8.bat"
set "ZIPALIGN=%BUILD_TOOLS%\zipalign.exe"
set "APKSIGNER=%BUILD_TOOLS%\apksigner.bat"
set "ANDROID_JAR=%ADB_DIR%\platforms\android-34\android.jar"
set "KEYSTORE=%ADB_DIR%\Project\debug.keystore"
set "OUT_DIR=..\bin"

if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"
if exist "build" rmdir /s /q "build"
mkdir "build"
mkdir "build\gen"
mkdir "build\obj"
mkdir "build\dex"

echo [1/6] Синхронизация веб-ассетов...
if not exist "assets" mkdir "assets"
copy /y "..\internal\web\static\*" "assets\" > nul

echo [2/6] Компиляция ресурсов (aapt2 compile)...
"%AAPT2%" compile --dir res -o build\compiled_res.zip
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Ошибка компиляции ресурсов aapt2
    exit /b 1
)

echo [3/6] Линковка APK и генерация R.java (aapt2 link)...
"%AAPT2%" link -I "%ANDROID_JAR%" --manifest AndroidManifest.xml build\compiled_res.zip -A assets --java build\gen -o build\base.apk
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Ошибка линковки aapt2
    exit /b 1
)

echo [4/6] Компиляция Java кода (javac)...
javac --release 8 -cp "%ANDROID_JAR%" -d build\obj src\com\node\control\MainActivity.java build\gen\com\node\control\R.java
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Ошибка компиляции Java
    exit /b 1
)

echo [5/6] Создание Dalvik Executable (d8)...
call "%D8%" --output build\dex --lib "%ANDROID_JAR%" build\obj\com\node\control\*.class
if %ERRORLEVEL% NEQ 0 (
    echo [ОШИБКА] Ошибка дексирования D8
    exit /b 1
)

cd build
copy /y "..\build\dex\classes.dex" "classes.dex" > nul
"%AAPT%" add base.apk classes.dex > nul
"%ZIPALIGN%" -f -p 4 base.apk aligned.apk
call "%APKSIGNER%" sign --ks "%KEYSTORE%" --ks-pass pass:android --out "%OUT_DIR%\Node.apk" aligned.apk
cd ..

if exist "%OUT_DIR%\Node.apk" (
    echo ==================================================
    echo [✓] APK успешно собран и подписан!
    echo Файл: %OUT_DIR%\Node.apk
    echo ==================================================
) else (
    echo [ОШИБКА] Не удалось создать финальный APK файл.
    exit /b 1
)
