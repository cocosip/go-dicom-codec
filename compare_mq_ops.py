#!/usr/bin/env python3
"""Compare MQ encoder and decoder operations"""

import re
import sys

def parse_mq_operations(filename):
    """Parse MQ operations from test output"""
    encoder_ops = []
    decoder_ops = []

    with open(filename, 'r') as f:
        for line in f:
            # Match encoder operations
            enc_match = re.match(r'\[MQE #(\d+) (BEFORE|AFTER)\](.+)', line)
            if enc_match:
                op_num = int(enc_match.group(1))
                phase = enc_match.group(2)
                details = enc_match.group(3).strip()
                encoder_ops.append((op_num, phase, details))

            # Match decoder operations
            dec_match = re.match(r'\[MQC #(\d+) (BEFORE|AFTER)\](.+)', line)
            if dec_match:
                op_num = int(dec_match.group(1))
                phase = dec_match.group(2)
                details = dec_match.group(3).strip()
                decoder_ops.append((op_num, phase, details))

    return encoder_ops, decoder_ops

def main():
    # Run test and capture output
    import subprocess
    result = subprocess.run(
        ['go', 'test', '-run', 'TestSimple5x5Debug'],
        cwd='jpeg2000/t1',
        capture_output=True,
        text=True
    )

    # Save to temp file
    with open('/tmp/mq_test_output.txt', 'w') as f:
        f.write(result.stdout)
        f.write(result.stderr)

    encoder_ops, decoder_ops = parse_mq_operations('/tmp/mq_test_output.txt')

    print(f"Encoder operations: {len(encoder_ops)//2}")
    print(f"Decoder operations: {len(decoder_ops)//2}")
    print()

    # Compare first 50 operations
    max_ops = min(50, len(encoder_ops)//2, len(decoder_ops)//2)

    for i in range(max_ops):
        enc_before = encoder_ops[i*2]
        enc_after = encoder_ops[i*2+1]
        dec_before = decoder_ops[i*2]
        dec_after = decoder_ops[i*2+1]

        print(f"=== Operation #{i+1} ===")
        print(f"ENC BEFORE: {enc_before[2]}")
        print(f"ENC AFTER:  {enc_after[2]}")
        print(f"DEC BEFORE: {dec_before[2]}")
        print(f"DEC AFTER:  {dec_after[2]}")

        # Check if states match after operation
        enc_state_match = re.search(r'newState=(\d+) newMPS=(\d+)', enc_after[2])
        dec_state_match = re.search(r'newState=(\d+) newMPS=(\d+)', dec_after[2])

        if enc_state_match and dec_state_match:
            enc_state = (enc_state_match.group(1), enc_state_match.group(2))
            dec_state = (dec_state_match.group(1), dec_state_match.group(2))

            if enc_state != dec_state:
                print(f"❌ STATE MISMATCH! Enc:{enc_state} Dec:{dec_state}")
                print()
                break
            else:
                print(f"✓ States match: {enc_state}")

        print()

if __name__ == '__main__':
    main()
