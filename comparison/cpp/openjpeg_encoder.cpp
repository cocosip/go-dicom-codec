// OpenJPEG-based JPEG2000 encoder for raw pixel data comparison
// Compile with: g++ -o openjpeg_encoder openjpeg_encoder.cpp -I../fo-dicom-codec-code/Native/Common/OpenJPEG ../fo-dicom-codec-code/Native/Common/OpenJPEG/*.c -lm -lpthread
// Usage: openjpeg_encoder <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

// OpenJPEG headers
extern "C" {
#include "openjpeg.h"
}

static void error_callback(const char *msg, void *client_data) {
    fprintf(stderr, "[ERROR] %s", msg);
}

static void warning_callback(const char *msg, void *client_data) {
    fprintf(stderr, "[WARNING] %s", msg);
}

static void info_callback(const char *msg, void *client_data) {
    fprintf(stdout, "[INFO] %s", msg);
}

int main(int argc, char *argv[]) {
    if (argc < 7) {
        fprintf(stderr, "Usage: %s <input.bin> <output.j2k> <width> <height> <components> <bitdepth> [signed]\n", argv[0]);
        fprintf(stderr, "Example: %s input.bin output.j2k 512 512 3 8 0\n", argv[0]);
        return 1;
    }

    const char *input_file = argv[1];
    const char *output_file = argv[2];
    int width = atoi(argv[3]);
    int height = atoi(argv[4]);
    int num_components = atoi(argv[5]);
    int bit_depth = atoi(argv[6]);
    int is_signed = (argc > 7) ? atoi(argv[7]) : 0;

    printf("OpenJPEG Encoder Configuration:\n");
    printf("  Input: %s\n", input_file);
    printf("  Output: %s\n", output_file);
    printf("  Dimensions: %dx%d\n", width, height);
    printf("  Components: %d\n", num_components);
    printf("  Bit depth: %d\n", bit_depth);
    printf("  Signed: %s\n", is_signed ? "yes" : "no");

    // Read raw pixel data
    FILE *fp = fopen(input_file, "rb");
    if (!fp) {
        fprintf(stderr, "Failed to open input file: %s\n", input_file);
        return 1;
    }

    fseek(fp, 0, SEEK_END);
    long file_size = ftell(fp);
    fseek(fp, 0, SEEK_SET);

    int bytes_per_sample = (bit_depth <= 8) ? 1 : 2;
    long expected_size = width * height * num_components * bytes_per_sample;

    printf("  File size: %ld bytes\n", file_size);
    printf("  Expected size: %ld bytes\n", expected_size);

    if (file_size < expected_size) {
        fprintf(stderr, "Warning: File size (%ld) is smaller than expected (%ld)\n", file_size, expected_size);
    }

    uint8_t *raw_data = (uint8_t *)malloc(file_size);
    if (!raw_data) {
        fprintf(stderr, "Failed to allocate memory for raw data\n");
        fclose(fp);
        return 1;
    }

    size_t read_size = fread(raw_data, 1, file_size, fp);
    fclose(fp);

    if (read_size != file_size) {
        fprintf(stderr, "Failed to read complete file\n");
        free(raw_data);
        return 1;
    }

    // Create OpenJPEG image structure
    opj_image_cmptparm_t *cmptparm = (opj_image_cmptparm_t *)calloc(num_components, sizeof(opj_image_cmptparm_t));
    if (!cmptparm) {
        fprintf(stderr, "Failed to allocate component parameters\n");
        free(raw_data);
        return 1;
    }

    for (int i = 0; i < num_components; i++) {
        cmptparm[i].dx = 1;
        cmptparm[i].dy = 1;
        cmptparm[i].w = width;
        cmptparm[i].h = height;
        cmptparm[i].x0 = 0;
        cmptparm[i].y0 = 0;
        cmptparm[i].prec = bit_depth;
        cmptparm[i].bpp = bit_depth;
        cmptparm[i].sgnd = is_signed;
    }

    OPJ_COLOR_SPACE color_space = (num_components == 1) ? OPJ_CLRSPC_GRAY : OPJ_CLRSPC_SRGB;
    opj_image_t *image = opj_image_create(num_components, cmptparm, color_space);
    free(cmptparm);

    if (!image) {
        fprintf(stderr, "Failed to create OpenJPEG image\n");
        free(raw_data);
        return 1;
    }

    image->x0 = 0;
    image->y0 = 0;
    image->x1 = width;
    image->y1 = height;

    // Convert raw data to OpenJPEG format (interleaved to planar)
    int total_pixels = width * height;

    if (bytes_per_sample == 1) {
        // 8-bit data
        for (int c = 0; c < num_components; c++) {
            for (int i = 0; i < total_pixels; i++) {
                image->comps[c].data[i] = raw_data[i * num_components + c];
            }
        }
    } else {
        // 16-bit data (little-endian)
        uint16_t *raw_data_16 = (uint16_t *)raw_data;
        for (int c = 0; c < num_components; c++) {
            for (int i = 0; i < total_pixels; i++) {
                image->comps[c].data[i] = raw_data_16[i * num_components + c];
            }
        }
    }

    free(raw_data);

    // Setup encoder parameters
    opj_cparameters_t parameters;
    opj_set_default_encoder_parameters(&parameters);

    // JPEG2000 lossless compression settings
    parameters.tcp_numlayers = 1;
    parameters.cp_disto_alloc = 1;
    parameters.tcp_rates[0] = 0; // lossless
    parameters.irreversible = 0; // reversible 5/3 wavelet
    parameters.numresolution = 6; // 5 decomposition levels
    parameters.cblockw_init = 64;
    parameters.cblockh_init = 64;

    // Set code format to J2K
    parameters.cod_format = 0; // J2K codestream

    // Create codec
    opj_codec_t *codec = opj_create_compress(OPJ_CODEC_J2K);
    if (!codec) {
        fprintf(stderr, "Failed to create encoder codec\n");
        opj_image_destroy(image);
        return 1;
    }

    // Set event handlers
    opj_set_info_handler(codec, info_callback, NULL);
    opj_set_warning_handler(codec, warning_callback, NULL);
    opj_set_error_handler(codec, error_callback, NULL);

    // Setup encoder
    if (!opj_setup_encoder(codec, &parameters, image)) {
        fprintf(stderr, "Failed to setup encoder\n");
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        return 1;
    }

    // Open output stream
    opj_stream_t *stream = opj_stream_create_default_file_stream(output_file, OPJ_FALSE);
    if (!stream) {
        fprintf(stderr, "Failed to create output stream\n");
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        return 1;
    }

    // Encode
    printf("\nEncoding...\n");
    if (!opj_start_compress(codec, image, stream)) {
        fprintf(stderr, "Failed to start compression\n");
        opj_stream_destroy(stream);
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        return 1;
    }

    if (!opj_encode(codec, stream)) {
        fprintf(stderr, "Failed to encode image\n");
        opj_stream_destroy(stream);
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        return 1;
    }

    if (!opj_end_compress(codec, stream)) {
        fprintf(stderr, "Failed to end compression\n");
        opj_stream_destroy(stream);
        opj_destroy_codec(codec);
        opj_image_destroy(image);
        return 1;
    }

    // Cleanup
    opj_stream_destroy(stream);
    opj_destroy_codec(codec);
    opj_image_destroy(image);

    printf("Encoding completed successfully!\n");
    printf("Output file: %s\n", output_file);

    return 0;
}
