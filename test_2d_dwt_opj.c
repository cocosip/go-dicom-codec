#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

// 从OpenJPEG复制的1D DWT函数
void opj_dwt_encode_1d(int32_t* data, int32_t* tmp, int width) {
    int sn, dn, i;
    sn = (width + 1) / 2;
    dn = width / 2;

    // Deinterleave
    for (i = 0; i < sn; i++) {
        tmp[i] = data[2 * i];
    }
    for (i = 0; i < dn; i++) {
        tmp[sn + i] = data[2 * i + 1];
    }

    // Predict
    for (i = 0; i < sn - 1; i++) {
        tmp[sn + i] = tmp[sn + i] - ((tmp[i] + tmp[i + 1]) >> 1);
    }
    if ((width % 2) == 0) {
        tmp[sn + i] = tmp[sn + i] - tmp[i];
    }

    // Update
    tmp[0] += (tmp[sn] + tmp[sn] + 2) >> 2;
    for (i = 1; i < dn; i++) {
        tmp[i] = tmp[i] + ((tmp[sn + (i - 1)] + tmp[sn + i] + 2) >> 2);
    }
    if ((width % 2) == 1) {
        tmp[i] = tmp[i] + ((tmp[sn + (i - 1)] + tmp[sn + (i - 1)] + 2) >> 2);
    }

    // Copy back
    memcpy(data, tmp, width * sizeof(int32_t));
}

// 2D DWT (简化版)
void opj_dwt_encode_2d(int32_t* data, int width, int height) {
    int32_t* row = (int32_t*)malloc(width * sizeof(int32_t));
    int32_t* col = (int32_t*)malloc(height * sizeof(int32_t));
    int32_t* tmp = (int32_t*)malloc(width > height ? width * sizeof(int32_t) : height * sizeof(int32_t));

    // 水平变换（行）
    for (int y = 0; y < height; y++) {
        memcpy(row, &data[y * width], width * sizeof(int32_t));
        opj_dwt_encode_1d(row, tmp, width);
        memcpy(&data[y * width], row, width * sizeof(int32_t));
    }

    // 垂直变换（列）
    for (int x = 0; x < width; x++) {
        for (int y = 0; y < height; y++) {
            col[y] = data[y * width + x];
        }
        opj_dwt_encode_1d(col, tmp, height);
        for (int y = 0; y < height; y++) {
            data[y * width + x] = col[y];
        }
    }

    free(row);
    free(col);
    free(tmp);
}

int main() {
    printf("========================================\n");
    printf("OpenJPEG 2D DWT测试 - 8x8图像\n");
    printf("========================================\n\n");

    int width = 8, height = 8;
    int32_t data[64] = {
        590, 590, 592, 591, 591, 591, 591, 590,
        589, 588, 586, 586, 587, 588, 588, 588,
        590, 591, 592, 591, 591, 591, 591, 590,
        589, 588, 586, 586, 587, 588, 588, 588,
        590, 591, 592, 591, 591, 591, 591, 590,
        589, 588, 586, 586, 587, 588, 588, 588,
        590, 591, 592, 591, 591, 591, 591, 590,
        589, 588, 586, 586, 587, 588, 588, 588,
    };

    printf("【原始8x8图像】\n");
    for (int y = 0; y < height; y++) {
        printf("Row %d: ", y);
        for (int x = 0; x < width; x++) {
            printf("%3d ", data[y*width+x]);
        }
        printf("\n");
    }

    // 应用2D DWT
    opj_dwt_encode_2d(data, width, height);

    printf("\n【DWT变换后】\n");
    printf("整个8x8数组:\n");
    for (int y = 0; y < height; y++) {
        printf("Row %d: ", y);
        for (int x = 0; x < width; x++) {
            printf("%4d ", data[y*width+x]);
        }
        printf("\n");
    }

    printf("\nLL子带 (4x4，左上角):\n");
    for (int y = 0; y < 4; y++) {
        for (int x = 0; x < 4; x++) {
            printf("%4d ", data[y*width+x]);
        }
        printf("\n");
    }

    printf("\nHL子带 (4x4，右上角):\n");
    for (int y = 0; y < 4; y++) {
        for (int x = 4; x < 8; x++) {
            printf("%4d ", data[y*width+x]);
        }
        printf("\n");
    }

    printf("\nLH子带 (4x4，左下角):\n");
    for (int y = 4; y < 8; y++) {
        for (int x = 0; x < 4; x++) {
            printf("%4d ", data[y*width+x]);
        }
        printf("\n");
    }

    printf("\nHH子带 (4x4，右下角):\n");
    for (int y = 4; y < 8; y++) {
        for (int x = 4; x < 8; x++) {
            printf("%4d ", data[y*width+x]);
        }
        printf("\n");
    }

    return 0;
}
