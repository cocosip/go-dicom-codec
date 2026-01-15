@echo off
REM JPEG2000 Encoder Comparison Script (Windows)
REM Usage: run_comparison.bat <input.bin> <width> <height> <components> <bitdepth> [signed]

setlocal enabledelayedexpansion

if "%~1"=="" (
    echo Usage: %~nx0 ^<input.bin^> ^<width^> ^<height^> ^<components^> ^<bitdepth^> [signed]
    echo Example: %~nx0 D:\1_raw_pixels.bin 512 512 3 8 0
    exit /b 1
)

set INPUT_FILE=%~1
set WIDTH=%2
set HEIGHT=%3
set COMPONENTS=%4
set BITDEPTH=%5
set SIGNED=%6

if "%SIGNED%"=="" set SIGNED=0

echo ============================================================
echo JPEG2000 Encoder Comparison
echo ============================================================
echo.
echo Input file: %INPUT_FILE%
echo Dimensions: %WIDTH%x%HEIGHT%
echo Components: %COMPONENTS%
echo Bit depth: %BITDEPTH%
echo Signed: %SIGNED%
echo.

REM Check if input file exists
if not exist "%INPUT_FILE%" (
    echo Error: Input file not found: %INPUT_FILE%
    exit /b 1
)

REM Step 1: Build OpenJPEG encoder if needed
echo [1/5] Building OpenJPEG encoder...
if not exist "cpp\openjpeg_encoder.exe" (
    pushd cpp
    g++ -o openjpeg_encoder.exe openjpeg_encoder.cpp -I..\..\fo-dicom-codec-code\Native\Common\OpenJPEG ..\..\fo-dicom-codec-code\Native\Common\OpenJPEG\*.c -lm -lpthread -lstdc++ 2>nul
    if errorlevel 1 (
        echo Error: Failed to build OpenJPEG encoder
        echo Please ensure you have g++ installed and OpenJPEG source files are available
        popd
        exit /b 1
    )
    popd
    echo   Built successfully!
) else (
    echo   Already built.
)
echo.

REM Step 2: Encode with OpenJPEG
echo [2/5] Encoding with OpenJPEG...
cpp\openjpeg_encoder.exe "%INPUT_FILE%" output_openjpeg.j2k %WIDTH% %HEIGHT% %COMPONENTS% %BITDEPTH% %SIGNED%
if errorlevel 1 (
    echo Error: OpenJPEG encoding failed
    exit /b 1
)
echo.

REM Step 3: Encode with Go
echo [3/5] Encoding with Go...
go run go_encoder.go "%INPUT_FILE%" output_go.j2k %WIDTH% %HEIGHT% %COMPONENTS% %BITDEPTH% %SIGNED%
if errorlevel 1 (
    echo Error: Go encoding failed
    exit /b 1
)
echo.

REM Step 4: Basic comparison
echo [4/5] Comparing results (basic)...
go run compare_j2k.go output_openjpeg.j2k output_go.j2k
echo.

REM Step 5: Detailed comparison
echo [5/5] Comparing results (detailed)...
go run compare_j2k.go output_openjpeg.j2k output_go.j2k detailed > comparison_detailed.txt
echo   Detailed comparison saved to: comparison_detailed.txt
echo.

echo ============================================================
echo Comparison completed!
echo ============================================================
echo.
echo Output files:
echo   - output_openjpeg.j2k (OpenJPEG encoded)
echo   - output_go.j2k (Go encoded)
echo   - comparison_detailed.txt (detailed analysis)
echo.

endlocal
