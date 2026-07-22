using FellowOakDicom;
using FellowOakDicom.Imaging;
using FellowOakDicom.Imaging.Codec;
using FellowOakDicom.Imaging.NativeCodec;

return NativeDecodeWorker.Run(args);

internal static class NativeDecodeWorker
{
    public static int Run(string[] args)
    {
        try
        {
            if (args.Length == 1 && IsHelp(args[0]))
            {
                PrintUsage();
                return 0;
            }

            if (args.Length != 3 || !string.Equals(args[0], "decode", StringComparison.OrdinalIgnoreCase))
            {
                PrintUsage();
                return 1;
            }

            var inputPath = TrimShellQuotes(args[1]);
            var outputPath = TrimShellQuotes(args[2]);
            if (!File.Exists(inputPath))
            {
                throw new FileNotFoundException("Input DICOM file was not found.", inputPath);
            }

            ConfigureDicomServices();
            var sourceFile = DicomFile.Open(inputPath, FileReadOption.ReadAll);
            var sourceSyntax = DicomPixelData.Create(sourceFile.Dataset).Syntax;
            var transcoder = new DicomTranscoder(sourceSyntax, DicomTransferSyntax.ExplicitVRLittleEndian);
            var decodedDataset = transcoder.Transcode(sourceFile.Dataset);
            Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(outputPath))!);
            new DicomFile(decodedDataset).Save(outputPath);
            Console.WriteLine($"NATIVE|pass|input={inputPath}|output={outputPath}");
            return 0;
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            Console.Error.WriteLine($"NATIVE|fail|{exception.GetType().Name}: {exception.Message}");
            return 1;
        }
    }

    private static void ConfigureDicomServices()
    {
        new DicomSetupBuilder()
            .RegisterServices(services => services
                .AddFellowOakDicom()
                .AddTranscoderManager<NativeTranscoderManager>())
            .Build();
    }

    private static bool IsHelp(string value) =>
        string.Equals(value, "--help", StringComparison.OrdinalIgnoreCase) ||
        string.Equals(value, "-h", StringComparison.OrdinalIgnoreCase) ||
        string.Equals(value, "/?", StringComparison.OrdinalIgnoreCase);

    private static string TrimShellQuotes(string value) => value.Trim().Trim('"', '\'');

    private static void PrintUsage()
    {
        Console.WriteLine("Usage:");
        Console.WriteLine("  fo-dicom-native-worker decode <input.dcm> <output.dcm>");
    }
}
