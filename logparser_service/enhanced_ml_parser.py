#!/usr/bin/env python3
"""
Enhanced ML-Based Log Parser
Uses multiple logparser algorithms for intelligent log parsing
"""

import os
import tempfile
import json
import re
from datetime import datetime
from typing import List, Dict, Any, Tuple
import pandas as pd

# Import available ML algorithms
from logparser import Drain, Spell, IPLoM, LogCluster, LenMa, LFA, LKE, LogMine, LogSig, Logram, SLCT, ULP, Brain, AEL

class EnhancedMLParser:
    """Enhanced ML-based log parser using multiple algorithms"""
    
    def __init__(self):
        self.algorithms = {
            'drain': Drain.LogParser,
            'spell': Spell.LogParser,
            'iplom': IPLoM.LogParser,
            'logcluster': LogCluster.LogParser,
            'lenma': LenMa.LogParser,
            'lfa': LFA.LogParser,
            'lke': LKE.LogParser,
            'logmine': LogMine.LogParser,
            'logsig': LogSig.LogParser,
            'logram': Logram.LogParser,
            'slct': SLCT.LogParser,
            'ulp': ULP.LogParser,
            'brain': Brain.LogParser,
            'ael': AEL.LogParser
        }
        
        # Algorithm configurations for different log types
        self.algorithm_configs = {
            'drain': {
                'depth': 4,
                'st': 0.5,
                'maxChild': 100
            },
            'spell': {
                'tau': 0.5,
                'regex': []
            },
            'iplom': {
                'rex': [],
                'k': 1,
                'threshold': 0.9
            },
            'logcluster': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'lenma': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'lfa': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'lke': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'logmine': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'logsig': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'logram': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'slct': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'ulp': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'brain': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            },
            'ael': {
                'rex': [],
                'support': 10,
                'rsupport': 0.1
            }
        }
    
    def detect_log_characteristics(self, lines: List[str]) -> Dict[str, Any]:
        """Analyze log characteristics to choose the best ML algorithm"""
        characteristics = {
            'total_lines': len(lines),
            'avg_line_length': sum(len(line) for line in lines) / len(lines) if lines else 0,
            'unique_lines': len(set(lines)),
            'duplicate_ratio': 1 - (len(set(lines)) / len(lines)) if lines else 0,
            'has_timestamps': any(re.search(r'\d{4}-\d{2}-\d{2}', line) for line in lines),
            'has_ips': any(re.search(r'\b(?:\d{1,3}\.){3}\d{1,3}\b', line) for line in lines),
            'has_hex': any(re.search(r'[0-9a-fA-F]{8,}', line) for line in lines),
            'has_uuids': any(re.search(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', line) for line in lines),
            'has_json': any(line.strip().startswith('{') and line.strip().endswith('}') for line in lines),
            'has_structured': any('=' in line or ':' in line for line in lines)
        }
        
        return characteristics
    
    def select_best_algorithm(self, characteristics: Dict[str, Any]) -> str:
        """Select the best ML algorithm based on log characteristics"""
        
        # If logs are mostly JSON, use simpler algorithm
        if characteristics['has_json']:
            return 'drain'
        
        # For highly structured logs with many duplicates
        if characteristics['duplicate_ratio'] > 0.7:
            return 'spell'
        
        # For logs with many unique patterns
        if characteristics.get('total_lines', 1) > 0 and characteristics['unique_lines'] / characteristics.get('total_lines', 1) > 0.8:
            return 'logmine'
        
        # For logs with timestamps and IPs (system logs)
        if characteristics['has_timestamps'] and characteristics['has_ips']:
            return 'iplom'
        
        # For logs with hex/UUID patterns
        if characteristics['has_hex'] or characteristics['has_uuids']:
            return 'logcluster'
        
        # Default to Drain for general purpose
        return 'drain'
    
    def parse_with_ml_algorithm(self, lines: List[str], algorithm_name: str, temp_dir: str) -> List[Dict[str, Any]]:
        """Parse logs using a specific ML algorithm, using only valid parameters for each algorithm"""
        # Filter out empty lines and normalize
        non_empty_lines = [line.strip() for line in lines if line.strip()]
        if not non_empty_lines:
            print(f"[WARN] No non-empty lines to parse with {algorithm_name}")
            return []
        
        # Create input file
        input_file = os.path.join(temp_dir, "input.log")
        with open(input_file, 'w') as f:
            for line in non_empty_lines:
                f.write(line + '\n')
        
        try:
            algorithm_class = self.algorithms[algorithm_name]
            config = self.algorithm_configs[algorithm_name]
            # --- Parameter selection by algorithm ---
            if algorithm_name == 'drain':
                parser = algorithm_class(
                    log_format='<Content>',
                    indir=temp_dir,
                    outdir=temp_dir,
                    depth=config['depth'],
                    st=config['st'],
                    maxChild=config['maxChild']
                )
            elif algorithm_name == 'spell':
                parser = algorithm_class(
                    log_format='<Content>',
                    indir=temp_dir,
                    outdir=temp_dir,
                    tau=config['tau'],
                    regex=config['regex']
                )
            elif algorithm_name == 'iplom':
                parser = algorithm_class(
                    log_format='<Content>',
                    indir=temp_dir,
                    outdir=temp_dir,
                    rex=config['rex'],
                    k=config['k'],
                    threshold=config['threshold']
                )
            elif algorithm_name == 'logcluster':
                parser = algorithm_class(
                    indir=temp_dir,
                    log_format='<Content>',
                    outdir=temp_dir,
                    rex=config['rex'],
                    support=config['support'],
                    rsupport=config['rsupport']
                )
            elif algorithm_name == 'lenma':
                parser = algorithm_class(
                    indir=temp_dir,
                    outdir=temp_dir,
                    log_format='<Content>',
                    threshold=config['rsupport'],  # Use rsupport as threshold for demo
                    rex=config['rex']
                )
            elif algorithm_name == 'logmine':
                parser = algorithm_class(
                    indir=temp_dir,
                    outdir=temp_dir,
                    log_format='<Content>',
                    k=config.get('k', 1),
                    rex=config['rex']
                )
            else:
                # For other algorithms, try minimal required params
                parser = algorithm_class(
                    indir=temp_dir,
                    outdir=temp_dir,
                    log_format='<Content>',
                    rex=config['rex']
                )
            # Parse logs
            parser.parse("input.log")
            # Read results
            structured_file = os.path.join(temp_dir, "input.log_structured.csv")
            if os.path.exists(structured_file):
                df = pd.read_csv(structured_file)
                return self.convert_ml_results_to_entries(df, non_empty_lines)
            else:
                print(f"[WARN] {algorithm_name} did not produce structured output")
                return self.fallback_parsing(non_empty_lines)
        except Exception as e:
            print(f"[ERROR] {algorithm_name} failed: {e}")
            return self.fallback_parsing(non_empty_lines)

    def convert_ml_results_to_entries(self, df: pd.DataFrame, original_lines: List[str]) -> List[Dict[str, Any]]:
        """Convert ML algorithm results to our standard format"""
        entries = []
        
        if 'Content' not in df.columns or 'EventTemplate' not in df.columns:
            print("[WARN] ML output is missing 'Content' or 'EventTemplate' columns. Using fallback.")
            return self.fallback_parsing(original_lines)
            
        for _, row in df.iterrows():
            content = row.get('Content', '')
            template = row.get('EventTemplate', '')
            entry = self.extract_structured_fields_from_ml_content(content, template)
            entries.append(entry)
            
        return entries

    def _extract_structured_data_from_line(self, line: str) -> Dict[str, Any]:
        """A centralized function to extract timestamp, level, and metadata from a single log line."""
        
        message = line
        timestamp = None
        level = None
        metadata = {}

        # 1. Extract Timestamp
        timestamp_patterns = [
            r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:Z|[+-]\d{2}:?\d{2})?',
            r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:?\d{2})?',
            r'\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+',
            r'\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}',
            r'\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)\]',
            r'\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]',
            r'\[(\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4})\]',
            r'\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2}',
        ]
        for pattern in timestamp_patterns:
            match = re.search(pattern, message)
            if match:
                ts_str = match.group(1) if len(match.groups()) > 0 else match.group(0)
                timestamp = self.normalize_timestamp(ts_str)
                message = message.replace(match.group(0), '', 1)
                break

        # 2. Extract Level
        level_patterns = [
            r'\[\s*(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|CRIT|TRACE)\s*\]',
            r'\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|CRIT|TRACE)\b',
            r'level[=:]\s*(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|CRIT|TRACE)',
        ]
        for pattern in level_patterns:
            match = re.search(pattern, message, re.IGNORECASE)
            if match:
                level_str = match.group(1).upper()
                if level_str in ["WARN", "WARNING"]: level = "WARN"
                elif level_str in ["ERR", "ERROR"]: level = "ERROR"
                elif level_str in ["CRIT", "CRITICAL", "FATAL"]: level = "FATAL"
                else: level = level_str
                message = message.replace(match.group(0), '', 1)
                break

        # 3. Extract Bracketed Metadata and Key-Value Pairs
        patterns = [
            r'(\b[a-zA-Z_][a-zA-Z0-9_]*=\S+)', # key=value
            r'\[([^\]]+)\]',  # Anything in brackets
        ]
        for pattern in patterns:
            matches = re.findall(pattern, message)
            for match_str in matches:
                full_match = f'[{match_str}]' if pattern == r'\[([^\]]+)\]' else match_str
                message = message.replace(full_match, '', 1)
                if '=' in match_str:
                    parts = match_str.split('=', 1)
                    if len(parts) == 2:
                        metadata[parts[0].strip()] = parts[1].strip()
                elif ':' in match_str:
                    sub_parts = re.split(r'\s*:\s*', match_str)
                    for part in sub_parts:
                        if ' ' in part.strip():
                             k, v = part.strip().split(' ', 1)
                             metadata[k] = v
                else:
                    metadata[f'tag_{len(metadata)+1}'] = match_str.strip()

        # 4. Final Cleanup
        message = re.sub(r'\|', '', message)
        message = re.sub(r'^\s*[:\s]*', '', message)
        message = re.sub(r'\s+', ' ', message).strip()

        # 5. Handle edge cases where message becomes empty
        if not message:
            # If we have a timestamp but no message, include the timestamp in message
            if timestamp and not level:
                message = line.strip()
            # If we have a level but no message, include the level in message  
            elif level and not timestamp:
                message = line.strip()
            # If we have both timestamp and level but no message, include both
            elif timestamp and level:
                message = line.strip()
        # Special case: if we have a level prefix but no timestamp, preserve the original line
        elif level and not timestamp and message != line.strip():
            message = line.strip()

        return {"timestamp": timestamp, "level": level, "message": message, "metadata": metadata}

    def extract_structured_fields_from_ml_content(self, content: str, template: str) -> Dict[str, Any]:
        """Extract structured fields from ML-parsed content using the centralized function."""
        
        entry_data = self._extract_structured_data_from_line(content)
        
        entry = {
            "timestamp": entry_data["timestamp"],
            "level": entry_data["level"],
            "message": entry_data["message"],
            "metadata": {
                "ml_template": template,
                "ml_confidence": 0.8,
                **entry_data["metadata"]
            },
            "rawData": content
        }
        return entry

    def normalize_timestamp(self, timestamp_str: str) -> str:
        """Normalize timestamp to ISO format"""
        if not timestamp_str:
            return None
        
        # Handle timestamps with timezone offsets like "+05:30"
        if re.search(r'[+-]\d{2}:\d{2}$', timestamp_str):
             # Remove colon for strptime
             ts_clean = timestamp_str[:-3] + timestamp_str[-2:]
             fmt = "%Y-%m-%dT%H:%M:%S.%f%z"
        else:
            ts_clean = timestamp_str.replace("Z", "")
            fmt = "%Y-%m-%dT%H:%M:%S.%f"

        formats = [
            fmt,
            "%Y-%m-%dT%H:%M:%S%z",
            "%Y-%m-%dT%H:%M:%S",
            "%Y-%m-%d %H:%M:%S.%f",
            "%Y-%m-%d %H:%M:%S",
            "%d/%b/%Y:%H:%M:%S %z" # for apache/nginx
        ]
        
        for f in formats:
            try:
                dt = datetime.strptime(ts_clean, f)
                return dt.isoformat() + "Z"
            except ValueError:
                continue
        
        try:
            # Fallback for ISO 8601 strings that python < 3.11 struggles with
            dt = datetime.fromisoformat(timestamp_str.replace("Z", "+00:00"))
            return dt.isoformat() + "Z"
        except (ValueError, TypeError):
            print(f"[WARN] Could not parse timestamp: {timestamp_str}")
            return timestamp_str # Return original if parsing fails

    def fallback_parsing(self, lines: List[str]) -> List[Dict[str, Any]]:
        """Fallback parsing using the centralized extraction function."""
        entries = []
        
        for line in lines:
            if not line.strip():
                continue
            
            entry_data = self._extract_structured_data_from_line(line)
            entry = {
                'timestamp': entry_data["timestamp"],
                'level': entry_data["level"],
                'message': entry_data["message"],
                'metadata': {
                    'ml_template': line,
                    'ml_confidence': 0.5,
                    'parsing_method': 'regex_fallback',
                    **entry_data["metadata"]
                },
                'rawData': line
            }
            entries.append(entry)
        
        return entries

    def stitch_multiline_logs(self, entries: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Stitch multiline log entries together (e.g., stack traces)"""
        if not entries:
            return []
            
        stitched_entries = []
        current_entry = None
        last_level = None  # Track the last seen level
        
        for entry in entries:
            # If a line has a timestamp, it's a new entry.
            # Also, if the ML model gave it a different template, treat as new entry.
            is_new_entry = entry.get("timestamp") is not None or \
                           (current_entry and entry.get("metadata", {}).get("ml_template") != 
                            current_entry.get("metadata", {}).get("ml_template"))

            if is_new_entry or not current_entry:
                if current_entry:
                    stitched_entries.append(current_entry)
                current_entry = entry
                # Update last_level if this entry has a level
                if entry.get("level"):
                    last_level = entry.get("level")
            else:
                # This is a continuation line, append its message.
                current_entry["message"] += "\n" + entry["message"]
                # Also append rawData
                current_entry["rawData"] += "\n" + entry["rawData"]
                # Merge metadata, giving precedence to the first line's values
                for k, v in entry["metadata"].items():
                    if k not in current_entry["metadata"]:
                        current_entry["metadata"][k] = v

        if current_entry:
            stitched_entries.append(current_entry)
            
        # Now propagate levels to entries that don't have them
        for i, entry in enumerate(stitched_entries):
            if not entry.get("level") and i > 0:
                # Inherit level from previous entry
                entry["level"] = stitched_entries[i-1].get("level")
            
        return stitched_entries

    def _is_continuation_line(self, message: str) -> bool:
        """
        Determine if a log line is a continuation of the previous line.
        This logic is now primarily handled by checking for a timestamp in stitch_multiline_logs.
        This function remains for potential future use or more complex heuristics.
        """
        # A line is a continuation if it does NOT start with a recognizable log prefix.
        # This is a simplified heuristic. The primary logic is now in `stitch_multiline_logs`.
        
        # Patterns that indicate a NEW log line (not exhaustive)
        timestamp_patterns = [
            r'^\d{4}-\d{2}-\d{2}',  # 2023-10-27
            r'^\[\d{4}-\d{2}-\d{2}', # [2023-10-27
            r'^\d{2}/\w{3}/\d{4}', # 12/Oct/2023
            r'^\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}', # Oct 27 12:34:56
        ]
        
        level_patterns = [
            r'^\[(DEBUG|INFO|WARN|ERROR|FATAL|CRITICAL|CRIT|TRACE)\]',
            r'^(DEBUG|INFO|WARN|ERROR|FATAL|CRITICAL|CRIT|TRACE):',
        ]

        for pattern in timestamp_patterns + level_patterns:
            if re.match(pattern, message, re.IGNORECASE):
                return False # It's a new line
        
        # Lines that often start continuations (e.g., stack traces)
        continuation_starters = [
            r'^\s+', # Starts with whitespace
            r'^at\s', # Java stack trace
            r'^Traceback\s', # Python stack trace
            r'^Caused by:',
            r'^\w+Exception:',
        ]
        
        for pattern in continuation_starters:
            if re.match(pattern, message):
                return True

        return False # Default to assuming it's a new line

    def propagate_fields_across_entries(self, entries: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Propagate fields like trace_id or request_id to subsequent related log entries."""
        last_seen_ids = {}
        propagation_keys = ['trace_id', 'request_id', 'correlation_id', 'session_id', 'user_id', 'process_id']

        for entry in entries:
            # Update last seen IDs from current entry's metadata
            for key in propagation_keys:
                if key in entry.get('metadata', {}):
                    last_seen_ids[key] = entry['metadata'][key]
            
            # Propagate if not present
            for key, value in last_seen_ids.items():
                if key not in entry.get('metadata', {}):
                    entry['metadata'][key] = value

        return entries

    def parse_logs_intelligently(self, lines: List[str]) -> List[Dict[str, Any]]:
        """
        Main parsing function.
        - Detects log characteristics.
        - Selects the best ML algorithm.
        - Parses logs.
        - Stitches multiline entries.
        - Propagates context.
        """
        
        if not lines or not any(line.strip() for line in lines):
            print("[INFO] Received empty log content.")
            return []
            
        print("[INFO] Starting intelligent log parsing...")
        
        # 1. Detect characteristics
        characteristics = self.detect_log_characteristics(lines)
        print(f"[INFO] Log characteristics: {json.dumps(characteristics, indent=2)}")
        
        # 2. Select best algorithm
        algorithm_name = self.select_best_algorithm(characteristics)
        print(f"[INFO] Selected algorithm: {algorithm_name}")
        
        with tempfile.TemporaryDirectory() as temp_dir:
            # 3. Parse with ML algorithm (which includes fallback)
            ml_entries = self.parse_with_ml_algorithm(lines, algorithm_name, temp_dir)
            
            # Handle the case where ML parsing completely fails and returns nothing
            if not ml_entries:
                 print("[WARN] ML parsing returned no entries. Attempting fallback on all lines.")
                 ml_entries = self.fallback_parsing(lines)

        # 4. Stitch multiline logs (e.g., stack traces)
        # If ML returns only one entry for a multi-line file, it's likely failed.
        # Re-parse line by line as a fallback before stitching.
        if len(ml_entries) == 1 and len(lines) > 1:
            print("[WARN] ML returned single entry for multi-line input. Re-parsing line-by-line.")
            entries_for_stitching = self.fallback_parsing(lines)
        else:
            entries_for_stitching = ml_entries
            
        stitched_entries = self.stitch_multiline_logs(entries_for_stitching)
        
        # 5. Propagate contextual fields
        final_entries = self.propagate_fields_across_entries(stitched_entries)
        
        print(f"[INFO] Parsing complete. Found {len(final_entries)} log entries.")
        
        return final_entries

if __name__ == '__main__':
    # Example usage for testing
    parser = EnhancedMLParser()
    
    # Sample log lines (complex mix)
    log_lines = [
        '2023-03-15T10:30:00.123Z [INFO] aef873be-8dcb-4291-8633-595034a2b220 - Request started for endpoint /api/v1/users',
        '2023-03-15T10:30:00.200Z [DEBUG] aef873be-8dcb-4291-8633-595034a2b220 - Authenticating user_id=123',
        '2023-03-15T10:30:00.250Z [ERROR] aef873be-8dcb-4291-8633-595034a2b220 - Database connection failed',
        '    at new Connection (node_modules/mysql/lib/Connection.js:89:13)',
        '    at createConnection (node_modules/mysql/index.js:12:10)',
        '    ... 10 more lines',
        '2023-03-15T10:30:01.000Z [INFO] aef873be-8dcb-4291-8633-595034a2b220 - Request finished with status 500'
    ]
    
    parsed_logs = parser.parse_logs_intelligently(log_lines)
    
    # Print results in a readable format
    print(json.dumps(parsed_logs, indent=4))

# Global instance for microservice compatibility
ml_parser = EnhancedMLParser() 