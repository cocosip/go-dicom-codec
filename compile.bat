@echo off
gcc -o encode_openjpeg.exe encode_openjpeg.c ^
  -Ifo-dicom-codec-code/Native/Common/OpenJPEG ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/bio.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/cio.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/dwt.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/event.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/function_list.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/image.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/invert.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/j2k.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/jp2.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/mct.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/mqc.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/openjpeg.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/opj_clock.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/opj_malloc.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/pi.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/sparse_array.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/t1.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/t2.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/tcd.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/tgt.c ^
  fo-dicom-codec-code/Native/Common/OpenJPEG/thread.c ^
  -lm -O2

if %errorlevel% == 0 (
    echo 编译成功！
    encode_openjpeg.exe
) else (
    echo 编译失败
)
