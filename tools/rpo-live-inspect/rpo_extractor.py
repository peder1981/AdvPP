#!/usr/bin/env python3
"""
RPO Generic Extractor - Extract metadata and patterns from RPO files
without requiring decryption keys.

Usage:
    python3 rpo_extractor.py <rpo_file> [--verbose] [--output <dir>]
"""

import struct
import hashlib
import json
import math
import os
import sys
from collections import Counter
from pathlib import Path


class RPOExtractor:
    """Generic RPO file analyzer and extractor."""
    
    # Known magic bytes
    MAGIC_CUSTOM = b'APNSRM0419'
    MAGIC_TLPP = b'APNSRM0420'
    MAGIC_TTTM = b'APNSRM0421'
    
    def __init__(self, rpo_path):
        self.path = rpo_path
        self.data = None
        self.header = None
        self.footer = None
        self.content = None
        self.name = None
        self.magic = None
        
    def load(self):
        """Load and parse the RPO file."""
        with open(self.path, 'rb') as f:
            self.data = f.read()
        
        # Parse header (12 bytes)
        if len(self.data) < 46:
            raise ValueError("RPO file too small")
        
        self.header = self.data[:12]
        self.footer = self.data[-36:]
        self.content = self.data[12:-36]
        
        # Extract name from header
        name_bytes = self.header[4:20]
        self.name = name_bytes.split(b'\x00')[0].decode('ascii', errors='replace')
        
        # Extract magic from footer
        magic_bytes = self.footer[:10]
        self.magic = magic_bytes.decode('ascii', errors='replace')
        
        return self
    
    def get_info(self):
        """Get basic file information."""
        return {
            'path': str(self.path),
            'name': self.name,
            'magic': self.magic,
            'size': len(self.data),
            'content_size': len(self.content),
            'header_size': len(self.header),
            'footer_size': len(self.footer),
        }
    
    def analyze_entropy(self):
        """Calculate entropy of content."""
        byte_counts = Counter(self.content)
        total = len(self.content)
        
        entropy = 0.0
        for count in byte_counts.values():
            if count > 0:
                p = count / total
                entropy -= p * math.log2(p)
        
        return entropy
    
    def extract_strings(self, min_length=4):
        """Extract printable ASCII strings."""
        strings = []
        current = []
        
        for i, b in enumerate(self.content):
            if 32 <= b <= 126:  # Printable ASCII
                current.append(chr(b))
            else:
                if len(current) >= min_length:
                    text = ''.join(current)
                    # Filter useful strings
                    if not any(skip in text for skip in ['\\x', 'APNSRM', '\x00']):
                        strings.append({
                            'offset': i - len(current),
                            'text': text,
                            'length': len(text)
                        })
                current = []
        
        return strings
    
    def find_patterns(self, pattern_size=16, min_occurrences=5):
        """Find repeating patterns in content."""
        patterns = Counter()
        first_offset = {}
        
        for i in range(0, len(self.content) - pattern_size, pattern_size):
            chunk = self.content[i:i+pattern_size]
            hash_val = hashlib.md5(chunk).hexdigest()[:8]
            patterns[hash_val] += 1
            if hash_val not in first_offset:
                first_offset[hash_val] = i
        
        # Filter by minimum occurrences
        repeated = {k: v for k, v in patterns.items() if v >= min_occurrences}
        
        return [
            {
                'hash': k,
                'count': v,
                'first_offset': first_offset[k],
                'sample': self.content[first_offset[k]:first_offset[k]+pattern_size].hex()
            }
            for k, v in sorted(repeated.items(), key=lambda x: -x[1])
        ][:20]
    
    def analyze_byte_distribution(self):
        """Analyze byte value distribution."""
        byte_counts = Counter(self.content)
        total = len(self.content)
        
        distribution = []
        for byte_val, count in byte_counts.most_common(20):
            distribution.append({
                'byte': f'0x{byte_val:02X}',
                'count': count,
                'percentage': round(count * 100 / total, 2)
            })
        
        return distribution
    
    def detect_encryption(self):
        """Detect if content appears to be encrypted."""
        entropy = self.analyze_entropy()
        unique_bytes = len(Counter(self.content))
        
        # High entropy and full byte range suggests encryption
        is_encrypted = entropy > 7.5 and unique_bytes > 250
        
        return {
            'entropy': entropy,
            'unique_bytes': unique_bytes,
            'is_encrypted': is_encrypted,
            'confidence': 'high' if is_encrypted else 'low'
        }
    
    def extract_metadata(self):
        """Extract all metadata from RPO."""
        info = self.get_info()
        entropy_analysis = self.detect_encryption()
        byte_dist = self.analyze_byte_distribution()
        strings = self.extract_strings()
        patterns = self.find_patterns()
        
        return {
            'info': info,
            'encryption': entropy_analysis,
            'byte_distribution': byte_dist,
            'strings': strings[:50],  # Top 50 strings
            'patterns': patterns,
            'total_strings': len(strings),
        }
    
    def save_report(self, output_dir):
        """Save analysis report to JSON file."""
        metadata = self.extract_metadata()
        
        output_path = Path(output_dir) / f"{Path(self.path).stem}_analysis.json"
        with open(output_path, 'w') as f:
            json.dump(metadata, f, indent=2)
        
        return output_path


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description='RPO Generic Extractor')
    parser.add_argument('rpo_file', help='Path to RPO file')
    parser.add_argument('--verbose', '-v', action='store_true', help='Show detailed output')
    parser.add_argument('--output', '-o', help='Output directory for reports')
    
    args = parser.parse_args()
    
    # Load and analyze
    extractor = RPOExtractor(args.rpo_file)
    extractor.load()
    
    # Print basic info
    info = extractor.get_info()
    print(f"RPO: {info['name']}")
    print(f"Magic: {info['magic']}")
    print(f"Size: {info['size']:,} bytes ({info['size']/1024/1024:.2f} MB)")
    print(f"Content: {info['content_size']:,} bytes")
    print()
    
    # Encryption detection
    enc = extractor.detect_encryption()
    print(f"Encryption detected: {enc['is_encrypted']} (confidence: {enc['confidence']})")
    print(f"Entropy: {enc['entropy']:.2f} bits/byte")
    print(f"Unique bytes: {enc['unique_bytes']}/256")
    print()
    
    # Byte distribution
    if args.verbose:
        print("Top 10 bytes:")
        for b in extractor.analyze_byte_distribution()[:10]:
            print(f"  {b['byte']}: {b['count']:,} ({b['percentage']}%)")
        print()
    
    # Strings
    strings = extractor.extract_strings()
    print(f"Strings found: {len(strings)}")
    for s in strings[:20]:
        print(f"  +{s['offset']:6d}: {s['text'][:50]}")
    print()
    
    # Patterns
    if args.verbose:
        patterns = extractor.find_patterns()
        print(f"Repeated patterns: {len(patterns)}")
        for p in patterns[:10]:
            print(f"  {p['hash']}: {p['count']}x at +{p['first_offset']}")
        print()
    
    # Save report
    if args.output:
        os.makedirs(args.output, exist_ok=True)
        report_path = extractor.save_report(args.output)
        print(f"Report saved: {report_path}")


if __name__ == '__main__':
    main()
