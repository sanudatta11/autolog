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
            'avg_line_length': sum(len(line) for line in lines) / len(lines),
            'unique_lines': len(set(lines)),
            'duplicate_ratio': 1 - (len(set(lines)) / len(lines)),
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
        if characteristics['unique_lines'] / characteristics.get('total_lines', 1) > 0.8:
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
        
        for idx, row in df.iterrows():
            content = row.get('Content', '')
            event_id = row.get('EventId', '')
            event_template = row.get('EventTemplate', '')
            
            # Extract structured information from the ML results
            entry = self.extract_structured_fields_from_ml_content(content, event_template)
            entries.append(entry)
        
        return entries
    
    def extract_structured_fields_from_ml_content(self, content: str, template: str) -> Dict[str, Any]:
        """Extract structured fields from ML-parsed content using template matching"""
        entry = {
            "timestamp": None,
            "level": None,
            "message": content,
            "metadata": {
                "ml_template": template,
                "ml_confidence": 0.8  # Placeholder for ML confidence
            },
            "rawData": content
        }
        
        # Extract timestamp
        timestamp_patterns = [
            r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})',
            r'\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?',
            r'\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]',
            r'(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})',
        ]
        
        for pattern in timestamp_patterns:
            match = re.search(pattern, content)
            if match:
                ts_str = match.group(1) if len(match.groups()) > 0 else match.group(0)
                entry["timestamp"] = self.normalize_timestamp(ts_str)
                break
        
        # Extract level from content or template
        level_patterns = [
            r'\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\b',
            r'\[(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\]',
            r'level[=:]\s*(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)',
        ]
        
        for pattern in level_patterns:
            match = re.search(pattern, content, re.IGNORECASE)
            if match:
                level = match.group(1).upper()
                if level in ["WARN", "WARNING"]:
                    level = "WARN"
                elif level in ["ERR", "ERROR"]:
                    level = "ERROR"
                elif level in ["CRIT", "CRITICAL", "FATAL"]:
                    level = "FATAL"
                elif level in ["DBG", "DEBUG"]:
                    level = "DEBUG"
                elif level in ["INF", "INFO"]:
                    level = "INFO"
                entry["level"] = level
                break
        
        # Extract additional fields using enhanced patterns
        patterns = [
            (r'\b(?:\d{1,3}\.){3}\d{1,3}\b', 'ip_address'),
            (r'\b(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s]+)', 'http_path'),
            (r'\b(\d{3})\s+(?:OK|ERROR|NOT_FOUND|FORBIDDEN|UNAUTHORIZED|BAD_REQUEST|INTERNAL_SERVER_ERROR)', 'http_status'),
            (r'request_id[=:]\s*([a-zA-Z0-9-_]+)', 'request_id'),
            (r'correlation_id[=:]\s*([a-zA-Z0-9-_]+)', 'correlation_id'),
            (r'execution_time[=:]\s*([0-9.]+)', 'execution_time'),
            (r'error_code[=:]\s*([a-zA-Z0-9_-]+)', 'error_code'),
            (r'user[=:]\s*([a-zA-Z0-9_-]+)', 'user'),
            (r'container[=:]\s*([a-zA-Z0-9_-]+)', 'container'),
            (r'pod[=:]\s*([a-zA-Z0-9_-]+)', 'pod'),
            (r'namespace[=:]\s*([a-zA-Z0-9_-]+)', 'namespace'),
            (r'file[=:]\s*([^\s]+)', 'file_path'),
            (r'line[=:]\s*(\d+)', 'line_number'),
            (r'function[=:]\s*([a-zA-Z0-9_]+)', 'function_name'),
            (r'module[=:]\s*([a-zA-Z0-9_.]+)', 'module_name'),
            (r'process_id[=:]\s*(\d+)', 'process_id'),
            (r'thread_id[=:]\s*(\d+)', 'thread_id'),
            (r'duration[=:]\s*([0-9.]+)', 'duration'),
            (r'size[=:]\s*(\d+)', 'size'),
            (r'count[=:]\s*(\d+)', 'count'),
            (r'https?://[^\s]+', 'url'),
            (r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b', 'email'),
            (r'\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b', 'uuid'),
        ]
        
        for pattern, field_name in patterns:
            matches = re.findall(pattern, content, re.IGNORECASE)
            if matches:
                if len(matches) == 1:
                    entry["metadata"][field_name] = matches[0]
                else:
                    entry["metadata"][field_name] = matches
        
        return entry
    
    def normalize_timestamp(self, timestamp_str: str) -> str:
        """Normalize timestamp to ISO format"""
        if not timestamp_str:
            return None
        
        formats = [
            "%Y-%m-%dT%H:%M:%S.%fZ",
            "%Y-%m-%dT%H:%M:%SZ", 
            "%Y-%m-%dT%H:%M:%S.%f",
            "%Y-%m-%dT%H:%M:%S",
            "%Y-%m-%d %H:%M:%S.%f",
            "%Y-%m-%d %H:%M:%S",
            "%Y-%m-%d %H:%M:%S.%f%z",
            "%Y-%m-%d %H:%M:%S%z",
        ]
        
        ts_clean = timestamp_str.replace("Z", "").replace("+00:00", "")
        
        for fmt in formats:
            try:
                dt = datetime.strptime(ts_clean, fmt)
                return dt.isoformat()
            except ValueError:
                continue
        
        try:
            dt = datetime.fromisoformat(timestamp_str.replace("Z", "+00:00"))
            return dt.isoformat()
        except Exception:
            pass
        
        return timestamp_str
    
    def fallback_parsing(self, lines: List[str]) -> List[Dict[str, Any]]:
        """Fallback parsing using regex patterns when ML algorithms fail"""
        entries = []
        
        for line in lines:
            if not line.strip():
                # Skip empty lines
                continue
                
            entry = {
                'timestamp': self._extract_timestamp(line),
                'level': self._extract_level(line),
                'message': line,
                'metadata': {
                    'ml_template': line,
                    'ml_confidence': 0.5,
                    'parsing_method': 'regex_fallback'
                },
                'rawData': line
            }
            entries.append(entry)
        
        return entries
    
    def _extract_timestamp(self, line: str) -> str:
        """Extract timestamp from a log line"""
        timestamp_patterns = [
            r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})',
            r'\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?',
            r'\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]',
            r'(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})',
        ]
        
        for pattern in timestamp_patterns:
            match = re.search(pattern, line)
            if match:
                ts_str = match.group(1) if len(match.groups()) > 0 else match.group(0)
                return self.normalize_timestamp(ts_str)
        return None
    
    def _extract_level(self, line: str) -> str:
        """Extract log level from a log line"""
        level_patterns = [
            r'\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\b',
            r'\[(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\]',
            r'level[=:]\s*(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)',
        ]
        
        for pattern in level_patterns:
            match = re.search(pattern, line, re.IGNORECASE)
            if match:
                level = match.group(1).upper()
                if level in ["WARN", "WARNING"]:
                    level = "WARN"
                elif level in ["ERR", "ERROR"]:
                    level = "ERROR"
                elif level in ["CRIT", "CRITICAL", "FATAL"]:
                    level = "FATAL"
                elif level in ["DBG", "DEBUG"]:
                    level = "DEBUG"
                elif level in ["INF", "INFO"]:
                    level = "INFO"
                return level
        return None
    
    def stitch_multiline_logs(self, entries: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Stitch multi-line log entries together (e.g., stack traces, continuation lines). For log types where every entry has a timestamp and level (e.g., Docker logs), skip stitching. Also, if a timestamp-only line is followed by a message-only line, treat as continuation."""
        if not entries:
            return entries
        # If all entries have both timestamp and level, do not stitch (treat as single-line logs)
        if all(entry.get('timestamp') and entry.get('level') for entry in entries):
            return entries
        stitched_entries = []
        current_entry = None
        i = 0
        while i < len(entries):
            entry = entries[i]
            message = entry.get('message', '')
            timestamp = entry.get('timestamp')
            level = entry.get('level')
            # Check if this is a new log entry (has timestamp and level)
            is_new_entry = timestamp and level and not self._is_continuation_line(message)
            # Check if this is a continuation line
            is_continuation = self._is_continuation_line(message)
            # Special case: if we have a timestamp+ERROR, it's likely a new stack trace
            is_new_stack_trace = (timestamp and level == 'ERROR' and 
                                ('Exception' in message or 'Error' in message or 'Traceback' in message))
            # Check for timestamp-only line followed by message-only line
            next_entry = entries[i+1] if i+1 < len(entries) else None
            # Check if current message is essentially just a timestamp (no meaningful content)
            is_timestamp_only = (timestamp and 
                               (not message.strip() or 
                                message.strip() == timestamp or 
                                re.match(r'^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$', message.strip())))
            
            if (is_timestamp_only and next_entry and 
                not next_entry.get('timestamp') and 
                next_entry.get('message', '').strip()):
                # Merge next entry's message into this entry
                entry['message'] = next_entry['message']
                if next_entry.get('metadata'):
                    if 'metadata' not in entry:
                        entry['metadata'] = {}
                    entry['metadata'].update(next_entry['metadata'])
                # Skip the next entry
                i += 1
            if is_new_stack_trace:
                if current_entry:
                    stitched_entries.append(current_entry)
                current_entry = entry.copy()
            elif is_continuation and current_entry:
                current_entry['message'] += '\n' + message
                if entry.get('metadata'):
                    if 'metadata' not in current_entry:
                        current_entry['metadata'] = {}
                    current_entry['metadata'].update(entry['metadata'])
            elif is_new_entry:
                if current_entry:
                    stitched_entries.append(current_entry)
                current_entry = entry.copy()
            else:
                if current_entry and (is_continuation or not timestamp):
                    current_entry['message'] += '\n' + message
                    if entry.get('metadata'):
                        if 'metadata' not in current_entry:
                            current_entry['metadata'] = {}
                        current_entry['metadata'].update(entry['metadata'])
                else:
                    if current_entry:
                        stitched_entries.append(current_entry)
                    current_entry = entry.copy()
            i += 1
        if current_entry:
            stitched_entries.append(current_entry)
        return stitched_entries
    
    def _is_continuation_line(self, message: str) -> bool:
        """Determine if a line is a continuation of a previous log entry"""
        if not message or not message.strip():
            return False
        
        # Stack trace patterns - more comprehensive
        stack_trace_patterns = [
            r'^Traceback \(most recent call last\):',
            r'^\s+File ".*", line \d+, in .*',
            r'^\s+File ".*", line \d+',
            r'^\s+.*',
            r'^[A-Za-z]+Error:',
            r'^[A-Za-z]+Exception:',
            r'^\s*at .*',
            r'^\s*Caused by:',
            r'^\s*... \d+ more',
            r'^\s*Suppressed:',
            r'^\s*raise .*',
            r'^\s*Exception:',
            r'^\s*RuntimeError:',
            r'^\s*ValueError:',
            r'^\s*TypeError:',
            r'^\s*AttributeError:',
            r'^\s*IndexError:',
            r'^\s*KeyError:',
            r'^\s*ImportError:',
            r'^\s*ModuleNotFoundError:',
            r'^\s*FileNotFoundError:',
            r'^\s*PermissionError:',
            r'^\s*TimeoutError:',
            r'^\s*ConnectionError:',
            r'^\s*OSError:',
            r'^\s*IOError:',
        ]
        
        # Check if line matches stack trace patterns
        for pattern in stack_trace_patterns:
            if re.match(pattern, message.strip()):
                return True
        
        # Check for indented lines (likely continuations)
        if message.startswith(' ') or message.startswith('\t'):
            return True
        
        # Check for common continuation indicators
        continuation_indicators = [
            '...',
            'continued',
            'more',
            'stack trace',
            'caused by',
            'suppressed',
            'nested exception',
            'traceback',
            'exception',
            'error:',
            'failed',
            'failed:',
        ]
        
        message_lower = message.lower()
        for indicator in continuation_indicators:
            if indicator in message_lower:
                return True
        
        # Check for lines that start with common stack trace elements
        if re.match(r'^\s*[A-Z][a-z]+Error:', message):
            return True
        
        if re.match(r'^\s*[A-Z][a-z]+Exception:', message):
            return True
        
        return False

    def propagate_fields_across_entries(self, entries: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Propagate last seen timestamp and level to entries that lack them (for multi-line/ambiguous logs)"""
        last_timestamp = None
        last_level = None
        for entry in entries:
            if entry.get("timestamp"):
                last_timestamp = entry["timestamp"]
            else:
                entry["timestamp"] = last_timestamp
            if entry.get("level"):
                last_level = entry["level"]
            else:
                entry["level"] = last_level
        return entries

    def parse_logs_intelligently(self, lines: List[str]) -> List[Dict[str, Any]]:
        """Intelligently parse logs using ML algorithms"""
        
        if not lines:
            return []
        
        # Detect log characteristics
        characteristics = self.detect_log_characteristics(lines)
        
        # Select best algorithm
        best_algorithm = self.select_best_algorithm(characteristics)
        
        print(f"[ML] Detected characteristics: {characteristics}")
        print(f"[ML] Selected algorithm: {best_algorithm}")
        
        # Create temporary directory
        with tempfile.TemporaryDirectory() as temp_dir:
            # Try primary algorithm
            try:
                entries = self.parse_with_ml_algorithm(lines, best_algorithm, temp_dir)
                if entries:
                    print(f"[ML] Successfully parsed {len(entries)} entries with {best_algorithm}")
                    entries = self.propagate_fields_across_entries(entries)
                    entries = self.stitch_multiline_logs(entries)
                    return entries
            except Exception as e:
                print(f"[ML] Primary algorithm {best_algorithm} failed: {e}")
            
            # Try fallback algorithms
            fallback_algorithms = ['drain', 'spell', 'logcluster']
            for algo in fallback_algorithms:
                if algo != best_algorithm:
                    try:
                        entries = self.parse_with_ml_algorithm(lines, algo, temp_dir)
                        if entries:
                            print(f"[ML] Successfully parsed {len(entries)} entries with fallback {algo}")
                            entries = self.propagate_fields_across_entries(entries)
                            entries = self.stitch_multiline_logs(entries)
                            return entries
                    except Exception as e:
                        print(f"[ML] Fallback algorithm {algo} failed: {e}")
            
            # Final fallback to regex parsing
            print("[ML] All ML algorithms failed, using regex fallback")
            entries = self.fallback_parsing(lines)
            entries = self.propagate_fields_across_entries(entries)
            entries = self.stitch_multiline_logs(entries)
            return entries

# Global instance
ml_parser = EnhancedMLParser() 