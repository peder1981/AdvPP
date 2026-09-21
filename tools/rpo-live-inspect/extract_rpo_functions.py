#!/usr/bin/env python3
"""
Extract functions from RPO files using pattern matching.
Works offline without needing decryption keys.
"""

import sys
import re
import struct
import hashlib
from collections import Counter, defaultdict
from pathlib import Path

class RPOExtractor:
    """Extract information from RPO files without decryption."""
    
    # Known RPO magic bytes
    MAGIC_CUSTOM = b'\x56\xbd\xb6\x00'  # custom.rpo
    MAGIC_TLPP = b'\x80\x91\x3f\x00'    # tlpp.rpo
    MAGIC_TTTM120 = b'\xc7\xb9\xf0\x00'  # tttm120.rpo (patch)
    MAGIC_TTTM120_ORIG = b'\xa9\xb3\x6c\x16'  # tttm120.rpo (original)
    
    # APO function markers (observed patterns)
    APO_PATTERNS = [
        rb'U_[A-Z0-9_]{3,10}',  # User functions
        rb'[A-Z]{2,4}[0-9]{3,5}',  # Routine names
        rb'\bFunction\b',  # Function keyword
        rb'\bMethod\b',  # Method keyword
    ]
    
    def __init__(self, rpo_path):
        self.path = Path(rpo_path)
        self.data = self.path.read_bytes()
        self.info = {}
        self.functions = []
        self.classes = []
        self.strings = []
        
    def analyze(self):
        """Perform complete analysis of RPO file."""
        self._extract_header()
        self._extract_footer()
        self._analyze_content()
        self._find_functions()
        self._find_classes()
        self._extract_strings()
        return self._generate_report()
    
    def _extract_header(self):
        """Extract header information."""
        if len(self.data) < 28:
            return
            
        self.info['magic'] = self.data[:4].hex()
        self.info['name'] = self.data[4:12].decode('ascii', errors='replace').strip('\x00')
        self.info['sentinel'] = struct.unpack('<I', self.data[12:16])[0]
        self.info['flags'] = struct.unpack('<I', self.data[20:24])[0]
        self.info['size_field'] = struct.unpack('<I', self.data[24:28])[0]
        self.info['md5'] = hashlib.md5(self.data).hexdigest()
        self.info['size'] = len(self.data)
        
    def _extract_footer(self):
        """Extract footer information."""
        footer_pos = self.data.rfind(b'APNSRM')
        if footer_pos > 0 and footer_pos < len(self.data) - 36:
            self.info['footer_offset'] = footer_pos
            self.info['footer_magic'] = self.data[footer_pos:footer_pos+12]
            self.info['trailer_sha1'] = self.data[footer_pos+12:footer_pos+36].hex()
            self.info['content_size'] = footer_pos - 12
            
    def _analyze_content(self):
        """Analyze content statistics."""
        byte_counts = Counter(self.data)
        
        self.info['unique_bytes'] = len(byte_counts)
        self.info['zero_bytes'] = byte_counts.get(0, 0)
        self.info['zero_percent'] = byte_counts.get(0, 0) / len(self.data) * 100
        
        # Calculate entropy
        import math
        entropy = 0.0
        for count in byte_counts.values():
            p = count / len(self.data)
            if p > 0:
                entropy -= p * math.log2(p)
        self.info['entropy'] = entropy
        
    def _find_functions(self):
        """Find potential function names."""
        patterns = [
            rb'U_[A-Z0-9_]{3,10}',
            rb'[A-Z][A-Z0-9_]{5,10}',
            rb'\bFunction\s+[A-Z_][A-Z0-9_]*',
            rb'\bUser\s+Function\s+[A-Z_][A-Z0-9_]*',
        ]
        
        functions = set()
        for pattern in patterns:
            matches = re.findall(pattern, self.data)
            for m in matches:
                try:
                    func_name = m.decode('ascii', errors='replace')
                    # Clean up
                    func_name = re.sub(r'^U_?', '', func_name)
                    func_name = re.sub(r'\s*Function.*', '', func_name)
                    if len(func_name) >= 3 and func_name.isalnum():
                        functions.add(func_name)
                except:
                    pass
                    
        self.functions = sorted(functions)
        
    def _find_classes(self):
        """Find potential class names."""
        patterns = [
            rb'class\s+[A-Z][A-Za-z0-9_]+',
            rb'[A-Z][A-Za-z0-9_]{5,}(?:\s+from|\s+new)',
        ]
        
        classes = set()
        for pattern in patterns:
            matches = re.findall(pattern, self.data, re.IGNORECASE)
            for m in matches:
                try:
                    class_name = m.decode('ascii', errors='replace')
                    class_name = re.sub(r'\s+from.*', '', class_name)
                    class_name = re.sub(r'\s+new.*', '', class_name)
                    if len(class_name) >= 3:
                        classes.add(class_name.strip())
                except:
                    pass
                    
        self.classes = sorted(classes)
        
    def _extract_strings(self):
        """Extract meaningful strings."""
        # Find all ASCII strings
        raw_strings = re.findall(rb'[\x20-\x7e]{6,}', self.data)
        
        strings = []
        for s in raw_strings:
            try:
                s_str = s.decode('ascii', errors='replace')
                # Filter out garbage
                if any(c < ' ' or c > '~' for c in s_str):
                    continue
                # Keep meaningful strings
                if re.search(r'[A-Z]{3,}|[A-Z]_[A-Z]|\.prw|\.tlpp', s_str):
                    strings.append(s_str)
            except:
                pass
                
        self.strings = sorted(set(strings))
        
    def _generate_report(self):
        """Generate analysis report."""
        report = []
        report.append("=" * 80)
        report.append(f"RPO ANALYSIS REPORT: {self.path.name}")
        report.append("=" * 80)
        report.append("")
        
        # Basic info
        report.append("[BASIC INFO]")
        report.append(f"  Size: {self.info.get('size', 0):,} bytes")
        report.append(f"  MD5: {self.info.get('md5', 'N/A')}")
        report.append(f"  Magic: {self.info.get('magic', 'N/A')}")
        report.append(f"  Name: {self.info.get('name', 'N/A')}")
        report.append(f"  Entropy: {self.info.get('entropy', 0):.6f} bits/byte")
        report.append(f"  Unique bytes: {self.info.get('unique_bytes', 0)}/256")
        report.append(f"  Zero bytes: {self.info.get('zero_bytes', 0):,} ({self.info.get('zero_percent', 0):.2f}%)")
        report.append("")
        
        # Functions found
        report.append(f"[FUNCTIONS] ({len(self.functions)} found)")
        for func in self.functions[:100]:
            report.append(f"  - {func}")
        if len(self.functions) > 100:
            report.append(f"  ... and {len(self.functions) - 100} more")
        report.append("")
        
        # Classes found
        report.append(f"[CLASSES] ({len(self.classes)} found)")
        for cls in self.classes[:50]:
            report.append(f"  - {cls}")
        if len(self.classes) > 50:
            report.append(f"  ... and {len(self.classes) - 50} more")
        report.append("")
        
        # Notable strings
        report.append("[NOTABLE STRINGS]")
        notable = [s for s in self.strings if any(x in s.upper() for x in ['GET', 'SET', 'FUNC', 'METHOD', 'CLASS', 'PRW', 'TLPP'])]
        for s in notable[:50]:
            report.append(f"  {s}")
        report.append("")
        
        return "\n".join(report)


def main():
    if len(sys.argv) < 2:
        print("Usage: python extract_rpo_functions.py <rpo_file> [output_file]")
        sys.exit(1)
        
    rpo_path = sys.argv[1]
    output_path = sys.argv[2] if len(sys.argv) > 2 else None
    
    extractor = RPOExtractor(rpo_path)
    report = extractor.analyze()
    
    print(report)
    
    if output_path:
        Path(output_path).write_text(report)
        print(f"\nReport saved to: {output_path}")


if __name__ == "__main__":
    main()
