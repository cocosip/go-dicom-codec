# Multi-Layer JPEG 2000 调试进度报告

## 日期
2025-11-28

## 任务目标
修复JPEG 2000多层(multi-layer)编码/解码功能，支持TERMALL模式的lossless压缩。

## 最终成果总结

### ✅ 成功解决的问题

1. **PassLengths BaseOffset计算Bug** - 在tile_decoder_fixed.go中修复
2. **Packet边界Offset错乱** - 识别upfront byte-unstuffing破坏packet边界的根本原因
3. **BitReader智能处理Stuffed Bytes** - 实现Option A，在bit reading时自动跳过stuffed bytes
4. **Header Reading Safety Check Bug** - 修复导致header提前终止的错误逻辑
5. **Single-layer TERMALL Lossless** - 完美工作 (error=0)

### 📊 测试结果

```
总计: 30个测试PASS ✓，2个测试FAIL ✗

关键测试:
- TestTERMALLSingleLayer:                    PASS ✓ (error=0, lossless完美)
- TestMultiLayerEncoding:                    PASS ✓
- TestMultiLayerLossyEncoding:               PASS ✓
- TestMultiLayerLossy:                       FAIL ✗ (error=255, 新问题)
- TestMultiLayerDifferentQualities/1_layer:  PASS ✓
- TestMultiLayerDifferentQualities/2_layers: FAIL ✗ (新问题)
```

### 🔧 关键技术修复

1. **移除upfront byte-unstuffing**
2. **BitReader.physicalPos tracking** - 正确追踪物理字节位置
3. **readAndUnstuff函数** - 读取stuffed bytes并按目标长度unstuff
4. **移除header reading的错误safety check**

### ⚠️ 剩余问题

Multi-layer lossy解码完全失败（error=255），但这是**独立于byte-stuffing的新问题**，可能与：
- T1 decoder在lossy模式的问题
- Quality层分配
- Bitplane计算

需要单独的调试session解决。

## 已完成工作

### 1. T1层TERMALL支持 ✓
- 实现了`DecodeLayered`函数，支持按pass长度解码
- 所有T1单元测试通过
- TERMALL模式在单层编码下工作正常

### 2. T2层Pass长度传递 ✓
- 实现了`PrecinctCodeBlock`结构中的`PassLengths`字段
- Packet encoder正确传递pass长度信息到packet header
- Packet decoder正确读取pass长度信息

### 3. Packet Bitstream扩展 ✓
- 在packet body中编码PassLengths metadata (TERMALL模式)
- 格式: 1 byte (num passes) + N × 2 bytes (pass lengths)
- DataLength正确包含metadata + data的总长度

### 4. Packet Decoder多层Inclusion状态跟踪 ✓
- 使用`cbIncluded` map跟踪code-block在不同layer的inclusion状态
- 正确处理FirstInclusion标志

### 5. PassLengths BaseOffset计算Bug修复 ✓
- **Bug**: 在`tile_decoder_fixed.go:66`，baseOffset在append data之后计算，导致错误
- **修复**: 在append之前保存baseOffset: `baseOffset := len(existing.data)`
- **验证**: PassLengths正确累加为 [2 5 8 11 14 16 19 22 25 28 31 34 37 40 43]

## 当前问题

### 核心问题：Byte-Stuffing导致的Packet边界偏移错误 (ROOT CAUSE IDENTIFIED!)

#### 症状
- **Single-layer**: 完美工作，error=0 ✓
- **Multi-layer Layer 0**: 正确编码和解码 ✓
- **Multi-layer Layer 1+**: 解码完全失败 ✗
  - Encoder输出: DataLength=4, layerData=[0x00, 0x2f, 0x49, ...]
  - Decoder读取: DataLength=27860 (完全错误!)
  - 结果: 所有像素解码错误 (max error=251/255)

#### 测试结果
```
TestTERMALLSingleLayer:        PASS (error=0)
TestMultiLayerLossy:           FAIL (error=251)
TestMultiLayerLossyEncoding:   FAIL (error=255)
```

#### 调试发现

##### Layer 0 Packet (正常)
```
[ENCODE PACKET] Layer=0 Res=0 Comp=0 numCB=1
[PACKET ENC] LayerPasses=[16 27], totalPasses=16, newPasses=16, included=true
[PACKET ENC] layerData len=5, first bytes=e303ba
[PACKET ENC] DataLength=5
[PACKET ENC Header] Header length=5 bytes

[PACKET DEC] Decoding header at offset 0, byte=84
[PACKET DEC] First 10 bytes: 8400040014e303ba049f
[PACKET DEC] Inclusion bit=1 ✓
[PACKET DEC] FirstInclusion=true ✓
[PACKET DEC] Decoded DataLength=5 ✓
[PACKET DEC] After decoding, offset=10
[DATA ACCUM] cbData len=5, first bytes=e303ba ✓
```

##### Layer 1 Packet (错误)
```
[ENCODE PACKET] Layer=1 Res=0 Comp=0 numCB=1
[PACKET ENC] LayerPasses=[16 27], totalPasses=27, newPasses=11, included=true
[PACKET ENC] layerData len=4, first bytes=002f49
[PACKET ENC] DataLength=4
[PACKET ENC Header] Header length=4 bytes

[PACKET DEC] Decoding header at offset 547, byte=ad
[PACKET DEC] First 10 bytes: ad9a941280100040002f
[PACKET DEC] Inclusion bit=1 ✓
[PACKET DEC] FirstInclusion=false ✓
[PACKET DEC] Decoded DataLength=27860 ✗✗✗ (应该是4!)
[PACKET DEC] After decoding, offset=1028
[DATA ACCUM] cbData len=478, first bytes=128010 ✗ (应该是002f49!)
```

#### 根本原因分析 (UPDATED - 真正的根本原因)

**问题**: Decoder upfront byte-unstuffing导致packet边界offset错乱

##### 新发现 (2025-11-28 最新调试)

通过对比encoder写入和decoder读取的offset，发现了真正的问题：

**Encoder写入 (encoder.go with byte-stuffing)**:
```
Layer 0 Res 5: offset=404, headerLen=13 (stuffed), bodyLen=137 (stuffed), total=150
Layer 1 Res 0: offset=554 (= 404 + 150)
```

**Decoder读取 (after upfront removeByteStuffing)**:
```
Layer 0 Res 5: offset=401, headerLen=9 (unstuffed), bodyLen=137 (unstuffed), total=146
Layer 1 Res 0: offset=547 (= 401 + 146)
```

**Offset差异**: 554 - 547 = 7 bytes

**根本原因**:
1. Encoder写入的bitstream包含byte-stuffing (0xFF → 0xFF 0x00)
2. Layer 0 Res 5 header在stuffed状态下是13 bytes
3. Decoder在packet_decoder.go:89执行`removeByteStuffing(pd.data)`，将整个bitstream unstuff
4. Layer 0 Res 5 header在unstuffed状态下是9 bytes (移除了4个stuffed 0x00)
5. Decoder在unstuffed数据上跟踪offset，认为Layer 0 Res 5结束于offset 547
6. 但实际上Layer 1 Res 0在原始stuffed bitstream中的offset是554
7. **结果**: Decoder从offset 547读取Layer 1 Res 0 header，但实际header在554，中间7字节的数据被误读为header bits！

**Packet边界错乱示意**:
```
Stuffed bitstream:    [L0R5 Header: 13 bytes] [L0R5 Body: 137 bytes] [L1R0 Header starts at byte 554]
                      ^                                              ^
                      offset 404                                     offset 554

Unstuffed bitstream:  [L0R5 Header: 9 bytes] [L0R5 Body: 137 bytes] [... 4 mystery bytes ...] [L1R0 Header]
                      ^                                             ^                         ^
                      offset 401                                    offset 547                offset 551

Decoder reads:        offset=401 → 410 → 547 (expects L1R0 header here!)
Actual L1R0 header:                                                  offset 551 in unstuffed = 554 in stuffed
```

**为什么Single-Layer工作？**:
- 只有一个header被stuffed，累积误差小
- 最后一个packet后没有更多packets需要解码
- Decoder即使offset错误也能完成解码

**之前错误的理解**: Byte-stuffing被错误地应用到packet headers上

##### JPEG 2000标准要求
- ✓ Packet bodies (MQ-coded data): 需要byte-stuffing (0xFF → 0xFF 0x00)
- ✗ Packet headers: **不应该**byte-stuffing

##### 当前错误实现
在`encoder.go:675-677`:
```go
for _, packet := range packets {
    writeWithByteStuffing(buf, packet.Header)  // ✗ 错误！Header不应该byte-stuff
    writeWithByteStuffing(buf, packet.Body)     // ✓ 正确
}
```

在`packet_decoder.go:89-91`:
```go
// 对整个bitstream做byte-unstuffing
unstuffed := removeByteStuffing(pd.data)  // ✗ 错误！这会破坏header
pd.data = unstuffed
```

##### 为什么Single-Layer能工作但Multi-Layer失败？

**Single-Layer**:
- 只有一个packet header被byte-stuffed
- 即使bit对齐被破坏，累积误差很小
- 碰巧可以正确解码

**Multi-Layer**:
- Layer 0的packet header被byte-stuffed → bit位置偏移
- Layer 0经过5个resolution (Res 0-5) → 累积6次偏移
- Layer 1 Res 0的packet header在offset 547开始
- 由于累积的bit misalignment，decoder从错误的bit位置读取
- DataLength的16 bits从错误位置读取 → 得到27860而不是4

##### Bit对齐破坏示例
假设Layer 0 header有一个0xFF byte:
- **编码**: `... FF ...` → **byte-stuff** → `... FF 00 ...` (多了1 byte)
- **解码**: `... FF 00 ...` → **byte-unstuff** → `... FF ...` (恢复)

但问题是：
1. Header是bit-packed数据（不是byte-aligned）
2. 在bit流中间插入0x00会导致所有后续bits偏移8位
3. Unstuff时移除0x00，但bit reader的位置已经错乱
4. 多个packet累积后，bit misalignment变得严重

#### 实验验证

尝试完全禁用byte-stuffing:
```go
// encoder.go
buf.Write(packet.Header)
buf.Write(packet.Body)

// packet_decoder.go
pd.offset = 0  // 不做byte-unstuffing
```

**结果**: 解码失败 `unexpected marker: 0xFF76`
- 原因: MQ-coded data包含0xFF bytes，被误认为是JPEG 2000 markers
- 结论: Packet body确实需要byte-stuffing

## 解决方案 (UPDATED - 基于新的根本原因)

### 错误策略 (已尝试并rollback)

**Phase 1-3实现** (2025-11-28 尝试):
- ✗ 在packet encoder中pre-stuff code-block data
- ✗ 移除encoder.go中header的byte-stuffing
- ✗ 移除decoder中的upfront unstuffing
- **结果**: Single-layer破坏 (error=0 → 64), Multi-layer仍失败

**为什么失败？**: 这个策略假设问题是"header被错误stuffed"，但实际问题是"upfront unstuffing破坏offset tracking"。

### 正确的解决方案

**核心原则**: Decoder必须在stuffed bitstream上工作，不能upfront unstuff！

#### Option A: 保持Encoder不变，修改Decoder

**Encoder**: 保持现有实现 (对所有data包括header和body做byte-stuffing)

**Decoder修改**:
1. **移除upfront unstuffing**
   ```go
   // packet_decoder.go:89
   // REMOVE这一行:
   // unstuffed := removeByteStuffing(pd.data)
   // pd.data = unstuffed

   // 直接使用stuffed data:
   pd.offset = 0
   ```

2. **在bit reader中处理stuffed bytes**
   ```go
   // bitReader.readBit() needs to handle 0xFF 0x00 stuffing
   func (br *bitReader) readBit() (int, error) {
       if br.bytePos >= len(br.data) {
           return 0, fmt.Errorf("end of data")
       }

       // Skip stuffed 0x00 after 0xFF
       if br.bytePos+1 < len(br.data) && br.data[br.bytePos] == 0xFF && br.data[br.bytePos+1] == 0x00 {
           if br.bitPos == 0 {
               // At the start of 0xFF byte, skip the next 0x00
               // Read from 0xFF first
           }
       }

       bit := int((br.data[br.bytePos] >> (7 - br.bitPos)) & 1)
       br.bitPos++

       if br.bitPos == 8 {
           br.bitPos = 0
           br.bytePos++
           // Skip stuffed 0x00 if current byte was 0xFF
           if br.bytePos < len(br.data) && br.data[br.bytePos-1] == 0xFF && br.data[br.bytePos] == 0x00 {
               br.bytePos++ // Skip 0x00
           }
       }

       return bit, nil
   }
   ```

3. **或者：延迟unstuff (更简单)**
   - 读取packet header时按stuffed bytes读取
   - Header读完后，unstuff整个header用于bit parsing
   - Body按stuffed length读取，然后unstuff再传给T1 decoder

#### Option B: 不对Header做Byte-Stuffing (标准兼容)

查阅JPEG 2000标准 ISO/IEC 15444-1:2019:
- **Packet headers**: 应该不需要byte-stuffing (标准未明确要求)
- **Packet bodies (code-stream data)**: 需要byte-stuffing

**实现步骤**:
1. Encoder修改: 只对packet body做byte-stuffing
2. Decoder修改: 移除upfront unstuffing，只unstuff packet body
3. 好处: 简化header parsing，避免bit misalignment问题

### 推荐方案: Option B (不对Header Byte-Stuff)

理由:
- 符合JPEG 2000标准理解
- Header是bit-packed metadata，stuffing会破坏bit alignment
- Body是MQ-coded data，stuffing是必需的（防止marker confusion）
- 实现更简单，性能更好

### 实现步骤 (Option B) - 遇到复杂问题

#### Phase 1: Packet Encoder不对Header做Byte-Stuffing
- [x] `encodePacket`不对header应用byte-stuffing (header.Bytes()直接返回)
- [x] `encoder.go`: 直接写入packet.Header，不调用`writeWithByteStuffing`

#### 新问题发现: PassLengths与Byte-Stuffing的冲突

尝试实现Option B时发现了新的复杂性:

1. **Pass Lengths计算时机**:
   - PassLengths是在T1 encoder计算的，基于UNSTUFFED data
   - 如果在packet encoder中stuff data，PassLengths位置会错位

2. **DataLength encoding难题**:
   - Packet header中的DataLength必须在header encoding时确定
   - 但byte-stuffing overhead只有在写packet body时才知道
   - 无法在header encoding时预知stuffed length

3. **尝试的方案及问题**:
   - ✗ 方案A: 在header encoding时pre-stuff → PassLengths错位
   - ✗ 方案B: 在body encoding时stuff并更新DataLength → header已经编码完成，无法更新

#### 测试结果

Option B partial implementation:
```
TestTERMALLSingleLayer:        error=64 (from 254, improved but still wrong)
TestMultiLayerLossyEncoding:   PASS ✓
TestMultiLayerLossy:           error=251 (no improvement)
```

### 最终实现：Option A (bitReader智能处理stuffed bytes) ✓

#### 实现细节

1. **移除upfront byte-unstuffing**:
   ```go
   // packet_decoder.go:86
   // 不再执行: unstuffed := removeByteStuffing(pd.data)
   pd.offset = 0
   ```

2. **bitReader自动跳过stuffed bytes**:
   ```go
   func (br *bitReader) readBit() (int, error) {
       // 在读取每个byte的第一个bit前，检查前一个byte是否是0xFF
       if br.bitPos == 0 && br.bytePos > 0 {
           prevByte := br.data[br.bytePos-1]
           if prevByte == 0xFF && br.bytePos < len(br.data) && br.data[br.bytePos] == 0x00 {
               // 跳过stuffed 0x00 byte
               br.bytePos++
           }
       }
       // 正常读取bit
       ...
   }
   ```

3. **readAndUnstuff函数**:
   ```go
   // 读取stuffed bytes并unstuff，直到得到目标长度的unstuffed data
   func readAndUnstuff(data []byte, targetUnstuffedLen int) ([]byte, int) {
       result := make([]byte, 0, targetUnstuffedLen)
       i := 0
       for i < len(data) && len(result) < targetUnstuffedLen {
           result = append(result, data[i])
           if data[i] == 0xFF && i+1 < len(data) && data[i+1] == 0x00 {
               i += 2 // Skip stuffed 0x00
           } else {
               i++
           }
       }
       return result, i // Returns unstuffed data and bytes read from stuffed stream
   }
   ```

4. **Packet body reading**:
   ```go
   // DataLength是unstuffed长度
   cbData, bytesRead := readAndUnstuff(pd.data[pd.offset:], cbIncl.DataLength)
   pd.offset += bytesRead // Advance by stuffed bytes count
   ```

#### 测试结果 (Option A)

```
TestTERMALLSingleLayer:                     PASS ✓ (error=0)
TestMultiLayerEncoding:                     PASS ✓
TestMultiLayerLossyEncoding:                PASS ✓
TestMultiLayerLossy:                        FAIL (error=251) ✗
TestMultiLayerDifferentQualities/1_layer:   PASS ✓
TestMultiLayerDifferentQualities/2_layers:  FAIL ✗
```

### 关键Bug修复: Header Reading Safety Check

#### 问题发现

在调试过程中发现packet header reading时有一个错误的safety check：

```go
// decodePacketHeader line 386-388 (WRONG!)
if len(cbIncls) > 0 && dataLen == 0 {
    break  // Premature exit!
}
```

这导致在解码high resolutions时，如果第一个code-block的dataLen=0，loop会提前break，只读取部分code-blocks的header。

**影响**:
- Layer 0 Res 5应该读3个code-blocks，但只读了2个
- Header physical length错误（读9 bytes，应该13 bytes）
- 导致后续Layer 1的packet offset错位

#### 修复

移除错误的safety check：

```go
// decodePacketHeader - 移除错误的safety check
for i := 0; i < maxCodeBlocks; i++ {
    // ... read code-block header ...
    cbIncls = append(cbIncls, cbIncl)

    // NOTE: Removed incorrect safety check that would break on dataLen=0
    // In JPEG 2000, empty code-blocks are valid and we need to read all maxCodeBlocks
}
```

#### 修复后测试结果

```
TestTERMALLSingleLayer:                     PASS ✓ (error=0)
TestMultiLayerEncoding:                     PASS ✓
TestMultiLayerLossyEncoding:                PASS ✓
TestMultiLayerLossy:                        FAIL (error=255) ✗ (新问题，非byte-stuffing相关)
TestMultiLayerDifferentQualities/1_layer:   PASS ✓
TestMultiLayerDifferentQualities/2_layers:  FAIL ✗ (新问题，非byte-stuffing相关)

总计: 30个测试PASS，2个测试FAIL
```

**Layer 1 Res 0 packet header现在正确解码**:
- Offset 554 (正确！)
- Header bytes: `80100040` (正确！)
- NumPasses: 11 (正确！)
- DataLength: 4 (正确！)

#### 剩余问题

Multi-layer lossy测试仍然失败，但这是**新的问题**（error=255，所有pixels错误），不是byte-stuffing或packet boundary的问题。

可能的原因：
1. T1 decoder在multi-layer lossy模式下的问题
2. Quality层分配算法问题
3. Bitplane计算问题

这需要单独的调试session来解决。
- [ ] 确保packet.Body包含pre-stuffed data

#### Phase 2: 移除Header Byte-Stuffing
- [ ] `encoder.go`: 直接写入packet.Header，不调用`writeWithByteStuffing`
- [ ] 保持packet.Body写入（已经pre-stuffed）

#### Phase 3: 更新Decoder
- [ ] `packet_decoder.go`: 移除全局`removeByteStuffing`调用
- [ ] 在读取packet body后，对individual cbData unstuff
- [ ] 正确管理offset（读取用stuffed length，advance offset）

#### Phase 4: 测试验证
- [ ] 运行`TestTERMALLSingleLayer` - 应该仍然PASS
- [ ] 运行`TestMultiLayerLossy` - 应该PASS，error应接近0
- [ ] 运行所有JPEG 2000测试 - 确保没有regression

## 相关文件

### 需要修改的文件
1. `jpeg2000/encoder.go` (line 675-677) - Packet写入逻辑
2. `jpeg2000/t2/packet_encoder.go` (line 383-401) - 添加body byte-stuffing
3. `jpeg2000/t2/packet_decoder.go` (line 89-91) - 移除全局unstuffing
4. `jpeg2000/t2/packet_decoder.go` (line 229-239) - 添加body unstuffing

### 测试文件
1. `jpeg2000/termall_single_layer_test.go` - Single-layer TERMALL (PASS)
2. `jpeg2000/multi_layer_lossy_test.go` - Multi-layer lossy (FAIL)
3. `jpeg2000/multilayer_test.go::TestMultiLayerLossyEncoding` - Multi-layer lossy (FAIL)

## 技术细节

### Packet Header结构 (Simplified)
```
For each code-block:
  - 1 bit: Inclusion flag (1=included, 0=not included)
  - If included && FirstInclusion:
      - N bits: Zero bitplanes (unary: 0...01)
  - If included:
      - M bits: Number of passes (unary: 0...01)
      - 16 bits: Data length (fixed-length)
```

### Packet Body结构
```
For each included code-block:
  - If TERMALL:
      - 1 byte: Number of passes
      - N × 2 bytes: Pass lengths (big-endian uint16)
  - X bytes: MQ-coded data (需要byte-stuffing!)
```

### Unary Encoding
```
Value 1:  1
Value 2:  01
Value 3:  001
Value N:  (N-1)个0 + 1个1
```

### Byte-Stuffing规则
```
Original:  ... XX FF YY ...
Stuffed:   ... XX FF 00 YY ...  (在0xFF后插入0x00)

原因: 避免0xFF被误认为JPEG 2000 marker (0xFF??)
```

## Debug输出关键点

### Encoder侧
- `[PACKET ENC]`: Packet编码开始
- `LayerPasses`: 每层的累积pass数量
- `newPasses`: 本层新增的pass数量
- `layerData len`: 本层的增量数据长度
- `DataLength`: 编码到header的长度（可能包含metadata）

### Decoder侧
- `[PACKET DEC]`: Packet解码开始
- `offset`: 当前读取位置
- `First N bytes`: 原始bitstream bytes
- `Inclusion bit`: 解码的inclusion标志
- `FirstInclusion`: 是否首次包含此code-block
- `Decoded DataLength`: 从header解码的长度
- `cbData len`: 实际读取的数据长度

## 性能数据

### 当前测试结果 (64x64, 2-layer lossy)
```
Encoded size: 1143 bytes
Layer 0: 成功解码
Layer 1: 完全失败
Max error: 251/255 (98% pixels wrong)
Error count: 4087/4096
```

### 预期结果 (修复后)
```
Encoded size: ~1140 bytes
Layer 0: 成功解码
Layer 1: 成功解码
Max error: <100 (lossy允许一定误差)
Error count: <50%
```

## 参考资料

### JPEG 2000标准文档
- ISO/IEC 15444-1:2019 Annex B (Packet Structure)
- Annex B.10: Byte-stuffing for entropy-coded data
- Annex B.10.1: "Byte-stuffing is applied only to compressed image data"

### 相关Issue
- Byte-stuffing破坏packet header bit对齐
- Multi-layer累积误差导致DataLength解码错误
- PassLengths baseOffset计算错误 (已修复)

## 下次工作计划

1. **实现正确的byte-stuffing策略** (优先级: 最高)
   - 只对packet body stuffing
   - 修改encoder写入逻辑
   - 修改decoder读取逻辑

2. **验证multi-layer lossless (TERMALL)**
   - 确保PassLengths正确传递
   - 验证T1层正确使用pass lengths解码

3. **完整测试套件**
   - Single-layer lossless (已通过)
   - Multi-layer lossless (待测试)
   - Multi-layer lossy (当前失败)
   - 不同图像尺寸和layer数量

4. **性能优化**
   - 减少不必要的数据拷贝
   - 优化bit reader/writer性能

## 备注

- 当前所有debug输出都应该在最终版本中移除或改为条件编译
- Byte-stuffing问题是所有multi-layer失败的根本原因
- 单层模式碰巧可以工作，但multi-layer暴露了架构问题
- 修复需要较大重构，但改动点明确，风险可控

## 2025-11-28 下午更新 - 实现尝试和新发现

### Phase 1-3实现完成
按照预定计划实现了：
1. ✓ Packet encoder中对code-block data预先byte-stuffing
2. ✓ Encoder不再对packet header做byte-stuffing
3. ✓ Decoder移除全局byte-unstuffing，改为对packet body单独unstuff

### 测试结果 - 仍然失败
- `TestTERMALLSingleLayer`: FAIL (error=64，之前是PASS error=0)
- `TestMultiLayerLossy`: FAIL (error=251，与之前相同)
- `TestSingleLayerLossyEncoding`: PASS ✓

### 新发现的问题

#### 问题1: Byte-Stuffing时机不当
现在的实现在packet encoder中对`layerData`做byte-stuffing，但这导致：
- TERMALL metadata (PassLengths) 和 MQ-coded data混在一起
- Metadata不应该被byte-stuffed
- 但它们share同一个DataLength

#### 问题2: 数据流复杂性
JPEG 2000的packet body结构：
```
[Metadata (TERMALL)] + [MQ-coded data]
```
- Metadata: 不应byte-stuffing
- MQ-coded data: 必须byte-stuffing
- 但encoder在一起写入packet body

#### 根本矛盾
- 如果对整个packet body byte-stuff → metadata被错误stuffed
- 如果只对data部分byte-stuff → 需要在packet encoder中区分metadata和data
- 当前实现混淆了这两个部分

### 核心问题重新分析

**Multi-layer失败的真实原因可能不是byte-stuffing！**

Debug输出显示：
```
[PACKET ENC Layer=1] DataLength=4, layerData=002f49
[PACKET DEC Layer=1] Decoded DataLength=27860 (应该是4)
```

这个DataLength decoding错误太大了(4 vs 27860)，不像是简单的byte-stuffing问题。

**新假设**: Packet header的bit-packing本身有问题
- Encoder写入DataLength=4 (0x0004) as 16 bits
- Decoder从错误的bit position读取，得到0x6CD4 (27860)

如果bit position偏移了，比如偏移4 bits:
- 原本: ...0000 0000 0000 0100...
- 读取时: ...xxxx 0000 0000 0000 0100 xxxx...
- 从错误位置读16 bits可能得到完全不同的值

### 下一步调试方向

#### 方向1: 验证bit position
添加debug输出encoder写入DataLength时的exact bit position，以及decoder读取时的bit position。

#### 方向2: 简化测试
创建minimal test case:
- 单个code-block
- 固定DataLength
- 验证encoder写入和decoder读取的一致性

#### 方向3: 回滚byte-stuffing修改
由于byte-stuffing重构导致regression，应该：
1. 回滚到原始实现
2. 专注解决bit alignment问题
3. Byte-stuffing作为separate issue处理

---
最后更新: 2025-11-28 下午
状态: Phase 1-3实现完成但测试失败，需要重新评估方案
