#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// 包含OpenJPEG头文件
#include "openjpeg.h"

#define WIDTH 888
#define HEIGHT 459
#define BIT_DEPTH 16
#define NUM_COMPONENTS 1

void print_pixels_int16(const char* label, int16_t* pixels, int count) {
    printf("%s:\n", label);
    for (int i = 0; i < count; i++) {
        printf("  pixel[%2d] = %6d\n", i, pixels[i]);
    }
    printf("\n");
}

int main() {
    printf("========================================\n");
    printf("使用OpenJPEG编码器（详细日志）\n");
    printf("========================================\n\n");

    // 读取原始像素数据
    FILE* fp = fopen("D:\\pixel_data_raw.bin", "rb");
    if (!fp) {
        printf("错误: 无法打开文件\n");
        return 1;
    }

    fseek(fp, 0, SEEK_END);
    long file_size = ftell(fp);
    fseek(fp, 0, SEEK_SET);

    unsigned char* pixel_data = (unsigned char*)malloc(file_size);
    fread(pixel_data, 1, file_size, fp);
    fclose(fp);

    printf("【编码参数】\n");
    printf("Width: %d\n", WIDTH);
    printf("Height: %d\n", HEIGHT);
    printf("BitDepth: %d\n", BIT_DEPTH);
    printf("IsSigned: true (1)\n");
    printf("Components: %d\n", NUM_COMPONENTS);
    printf("Pixel data size: %ld bytes\n", file_size);
    printf("\n");

    // 打印前20个像素值
    printf("【原始像素数据】\n");
    printf("前20个像素 (as int16):\n");
    int16_t* pixels_int16 = (int16_t*)pixel_data;
    for (int i = 0; i < 20; i++) {
        printf("  pixel[%2d] = %6d\n", i, pixels_int16[i]);
    }
    printf("\n");

    // 计算统计信息
    int pixel_count = WIDTH * HEIGHT;
    int16_t min_val = 32767, max_val = -32768;
    for (int i = 0; i < pixel_count; i++) {
        if (pixels_int16[i] < min_val) min_val = pixels_int16[i];
        if (pixels_int16[i] > max_val) max_val = pixels_int16[i];
    }
    printf("像素值范围: [%d, %d]\n\n", min_val, max_val);

    // 创建OpenJPEG参数
    printf("【OpenJPEG编码参数】\n");

    opj_cparameters_t parameters;
    opj_set_default_encoder_parameters(&parameters);

    parameters.tcp_numlayers = 1;
    parameters.cp_fixed_quality = 1;
    parameters.tcp_distoratio[0] = 0; // lossless
    parameters.numresolution = 6; // 5 DWT levels + 1
    parameters.irreversible = 0; // reversible 5-3 wavelet

    printf("NumLevels (DWT分解层数): %d\n", parameters.numresolution - 1);
    printf("NumLayers (质量层数): %d\n", parameters.tcp_numlayers);
    printf("Irreversible (9-7 wavelet): %d (0=5-3 reversible)\n", parameters.irreversible);
    printf("\n");

    // DC Level Shift计算
    printf("【DC Level Shift】\n");
    int is_signed = 1; // true for our data
    int32_t dc_shift = 0;
    if (!is_signed) {
        dc_shift = 1 << (BIT_DEPTH - 1);
    }
    printf("IsSigned = %d\n", is_signed);
    printf("DC Shift = %d\n", dc_shift);
    printf("说明: OpenJPEG会在tcd_dc_level_shift_encode()中应用此值\n");
    printf("      对于signed数据，m_dc_level_shift = 0\n");
    printf("      对于unsigned数据，m_dc_level_shift = 2^(prec-1)\n");
    printf("\n");

    // 创建OpenJPEG image结构
    opj_image_cmptparm_t cmptparm;
    memset(&cmptparm, 0, sizeof(cmptparm));

    cmptparm.dx = 1;
    cmptparm.dy = 1;
    cmptparm.w = WIDTH;
    cmptparm.h = HEIGHT;
    cmptparm.x0 = 0;
    cmptparm.y0 = 0;
    cmptparm.prec = BIT_DEPTH;
    cmptparm.bpp = BIT_DEPTH;
    cmptparm.sgnd = 1; // signed

    printf("【OpenJPEG Image Component参数】\n");
    printf("Width: %d\n", cmptparm.w);
    printf("Height: %d\n", cmptparm.h);
    printf("Precision (prec): %d\n", cmptparm.prec);
    printf("Signed (sgnd): %d\n", cmptparm.sgnd);
    printf("\n");

    opj_image_t *image = opj_image_create(NUM_COMPONENTS, &cmptparm, OPJ_CLRSPC_GRAY);
    if (!image) {
        printf("错误: 无法创建OpenJPEG image\n");
        free(pixel_data);
        return 1;
    }

    image->x0 = 0;
    image->y0 = 0;
    image->x1 = WIDTH;
    image->y1 = HEIGHT;

    // 复制像素数据到OpenJPEG image结构
    printf("【复制像素数据到OpenJPEG】\n");
    printf("前20个像素值 (复制到image->comps[0].data):\n");
    for (int i = 0; i < pixel_count; i++) {
        // 将int16_t转换为int32_t
        image->comps[0].data[i] = (OPJ_INT32)pixels_int16[i];
        if (i < 20) {
            printf("  image->comps[0].data[%2d] = %6d\n", i, image->comps[0].data[i]);
        }
    }
    printf("\n");

    printf("【OpenJPEG编码过程】\n");
    printf("1. 输入数据已复制到image->comps[0].data\n");
    printf("2. OpenJPEG将执行:\n");
    printf("   - opj_tcd_dc_level_shift_encode(): 应用DC shift (对signed=0)\n");
    printf("   - opj_tcd_dwt_encode(): DWT变换\n");
    printf("   - opj_t1_encode_cblks(): T1编码（EBCOT）\n");
    printf("   - opj_t2_encode_packets(): T2编码（打包）\n");
    printf("\n");

    // 创建编码器
    opj_codec_t* codec = opj_create_compress(OPJ_CODEC_J2K);
    if (!codec) {
        printf("错误: 无法创建编码器\n");
        opj_image_destroy(image);
        free(pixel_data);
        return 1;
    }

    // 设置编码器参数
    if (!opj_setup_encoder(codec, &parameters, image)) {
        printf("错误: 无法设置编码器\n");
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        free(pixel_data);
        return 1;
    }

    // 【关键】检查setup_encoder后image->comps[0].data是否被修改
    printf("\n【setup_encoder后检查像素数据】\n");
    printf("前20个像素值 (setup后):\n");
    for (int i = 0; i < 20; i++) {
        printf("  image->comps[0].data[%2d] = %6d\n", i, image->comps[0].data[i]);
    }
    printf("\n");

    // 创建输出流
    opj_stream_t *stream = opj_stream_create_default_file_stream("D:\\encoded_openjpeg.j2k", OPJ_FALSE);
    if (!stream) {
        printf("错误: 无法创建输出流\n");
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        free(pixel_data);
        return 1;
    }

    // 编码
    printf("【开始编码】\n");
    printf("调用 opj_start_compress...\n");
    OPJ_BOOL success = opj_start_compress(codec, image, stream);

    if (success) {
        printf("调用 opj_encode...\n");
        success = opj_encode(codec, stream);
    }
    if (success) {
        printf("调用 opj_end_compress...\n");
        success = opj_end_compress(codec, stream);
    }

    if (success) {
        printf("✓ 编码成功\n");
        printf("输出文件: D:\\encoded_openjpeg.j2k\n");
    } else {
        printf("✗ 编码失败\n");
    }

    // 清理
    opj_stream_destroy(stream);
    opj_destroy_codec(codec);
    opj_image_destroy(image);
    free(pixel_data);

    printf("\n========================================\n");
    printf("编码完成\n");
    printf("========================================\n");

    return success ? 0 : 1;
}
