#!/usr/bin/env python3
"""
Comprehensive test suite for the enhanced ML logparser
Merges all test functionality with command-line flags
"""

import sys
import os
import argparse
import requests
import tempfile
import json
import time
from typing import List, Dict
from datetime import datetime

# Add current directory to path for imports
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

def add_test_case(category, test_case):
    """
    Add a new test case to the JSON file
    
    Args:
        category (str): Category of test case ('ml_parser_test_cases', 'microservice_test_cases', 'real_world_scenarios')
        test_case (dict): Test case dictionary with name, description, lines/content, and expect fields
    """
    try:
        import json
        
        # Load existing test cases
        try:
            with open('test_cases.json', 'r') as f:
                test_data = json.load(f)
        except FileNotFoundError:
            test_data = {
                "ml_parser_test_cases": [],
                "microservice_test_cases": [],
                "real_world_scenarios": []
            }
        
        # Add new test case
        if category not in test_data:
            test_data[category] = []
        
        test_data[category].append(test_case)
        
        # Save back to file
        with open('test_cases.json', 'w') as f:
            json.dump(test_data, f, indent=2)
        
        print(f"✓ Added test case '{test_case['name']}' to {category}")
        return True
        
    except Exception as e:
        print(f"❌ Error adding test case: {e}")
        return False

def list_test_cases():
    """List all available test cases from the JSON file"""
    try:
        import json
        with open('test_cases.json', 'r') as f:
            test_data = json.load(f)
        
        print("Available test cases:")
        print("=" * 50)
        
        for category, cases in test_data.items():
            print(f"\n{category.replace('_', ' ').title()}:")
            print("-" * 30)
            for i, case in enumerate(cases, 1):
                print(f"  {i}. {case['name']}")
                if 'description' in case:
                    print(f"     {case['description']}")
        
        return True
        
    except FileNotFoundError:
        print("❌ test_cases.json file not found")
        return False
    except Exception as e:
        print(f"❌ Error listing test cases: {e}")
        return False

def test_ml_parser():
    """Test the enhanced ML parser with various log types"""
    print("=== Enhanced ML Parser Test ===")
    
    try:
        from enhanced_ml_parser import EnhancedMLParser
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False
    
    # Initialize parser
    parser = EnhancedMLParser()
    
    # Load test cases from JSON file
    try:
        import json
        with open('test_cases.json', 'r') as f:
            test_data = json.load(f)
        test_cases = test_data.get('ml_parser_test_cases', [])
        if not test_cases:
            print("❌ No test cases found in test_cases.json")
            return False
    except FileNotFoundError:
        print("❌ test_cases.json file not found")
        return False
    except json.JSONDecodeError as e:
        print(f"❌ Invalid JSON in test_cases.json: {e}")
        return False
    
    success_count = 0
    total_tests = len(test_cases)
    
    # Test each log type from JSON
    for test_case in test_cases:
        print(f"Testing {test_case['name']} ({len(test_case['lines'])} lines):")
        print("-" * 50)
        
        try:
            # Test characteristic detection
            characteristics = parser.detect_log_characteristics(test_case['lines'])
            print(f"Detected characteristics: {characteristics}")
            
            # Test algorithm selection
            best_algorithm = parser.select_best_algorithm(characteristics)
            print(f"Selected algorithm: {best_algorithm}")
            
            # Test parsing
            entries = parser.parse_logs_intelligently(test_case['lines'])
            print(f"Parsed {len(entries)} entries")
            
            # Show sample results
            if entries:
                print("Sample entry:")
                sample = entries[0]
                print(f"  Timestamp: {sample.get('timestamp')}")
                print(f"  Level: {sample.get('level')}")
                print(f"  Message: {sample.get('message')[:100]}...")
                print(f"  Metadata keys: {list(sample.get('metadata', {}).keys())}")
                success_count += 1
            
            print()
            
        except Exception as e:
            print(f"ERROR testing {test_case['name']}: {e}")
            print()
    
    # Test algorithm availability
    print("=== Algorithm Availability Test ===")
    print(f"Available algorithms: {list(parser.algorithms.keys())}")
    
    # Test each algorithm individually
    for algo_name in parser.algorithms.keys():
        try:
            config = parser.algorithm_configs[algo_name]
            print(f"✓ {algo_name}: {config}")
        except Exception as e:
            print(f"✗ {algo_name}: {e}")
    
    print(f"\n=== ML Parser Test Complete: {success_count}/{total_tests} successful ===")
    return success_count == total_tests

def test_ml_only_analysis():
    """Test only the ML log analysis with a variety of log types, structured and unstructured, and verify output"""
    print("\n=== ML-Only Log Analysis Test ===")
    
    try:
        from enhanced_ml_parser import EnhancedMLParser
    except ImportError as e:
        print(f"❌ Import error: {e}")
        return False
    
    # Load test cases from JSON file
    try:
        import json
        with open('test_cases.json', 'r') as f:
            test_data = json.load(f)
        test_cases = test_data.get('ml_parser_test_cases', [])
        if not test_cases:
            print("❌ No test cases found in test_cases.json")
            return False
    except FileNotFoundError:
        print("❌ test_cases.json file not found")
        return False
    except json.JSONDecodeError as e:
        print(f"❌ Invalid JSON in test_cases.json: {e}")
        return False
    
    parser = EnhancedMLParser()
    
    success_count = 0
    total_tests = len(test_cases)
    
    for case in test_cases:
        print(f"Test: {case['name']}")
        if 'description' in case:
            print(f"  Description: {case['description']}")
        try:
            entries = parser.parse_logs_intelligently(case["lines"])
            for i, (entry, expect) in enumerate(zip(entries, case["expect"])):
                print(f"  Entry {i+1}: {entry}")
                # Check expected fields
                if "level" in expect:
                    assert entry["level"] == expect["level"], f"Expected level {expect['level']}, got {entry['level']}"
                if "message_contains" in expect:
                    assert expect["message_contains"] in entry["message"], f"Expected message to contain '{expect['message_contains']}', got '{entry['message']}'"
            print("  ✓ Output matches expected\n")
            success_count += 1
        except Exception as e:
            print(f"  ❌ Error: {e}\n")
    
    print(f"=== ML-Only Log Analysis Test Complete: {success_count}/{total_tests} successful ===")
    return success_count == total_tests

def test_microservice_parsing():
    """Test the logparser microservice with various log types"""
    print("\n=== Testing Logparser Microservice ===")
    
    base_url = "http://localhost:8001"
    
    # Load test cases from JSON file
    try:
        import json
        with open('test_cases.json', 'r') as f:
            test_data = json.load(f)
        test_cases = test_data.get('microservice_test_cases', [])
        if not test_cases:
            print("❌ No microservice test cases found in test_cases.json")
            return False
    except FileNotFoundError:
        print("❌ test_cases.json file not found")
        return False
    except json.JSONDecodeError as e:
        print(f"❌ Invalid JSON in test_cases.json: {e}")
        return False
    
    success_count = 0
    total_tests = len(test_cases)
    
    for i, test_case in enumerate(test_cases, 1):
        print(f"Test {i}: {test_case['name']}")
        if 'description' in test_case:
            print(f"  Description: {test_case['description']}")
        print("-" * 50)
        
        try:
            # Create temporary file
            with tempfile.NamedTemporaryFile(mode='w', suffix='.log', delete=False) as tmp_file:
                tmp_file.write(test_case['content'])
                tmp_file_path = tmp_file.name
            
            # Send file to microservice
            with open(tmp_file_path, 'rb') as f:
                files = {'file': (f'test_{i}.log', f, 'text/plain')}
                response = requests.post(f"{base_url}/parse", files=files)
            
            # Clean up temp file
            os.unlink(tmp_file_path)
            
            if response.status_code == 200:
                result = response.json()
                print(f"✓ Success: Parsed {len(result)} entries")
                
                # Show sample entry
                if result:
                    sample = result[0]
                    print(f"Sample entry:")
                    print(f"  Timestamp: {sample.get('timestamp')}")
                    print(f"  Level: {sample.get('level')}")
                    print(f"  Message: {sample.get('message', '')[:100]}...")
                    if sample.get('metadata'):
                        print(f"  Metadata keys: {list(sample['metadata'].keys())}")
                
                # Verify expected fields
                if result:
                    sample = result[0]
                    missing_fields = []
                    for field in test_case['expected_fields']:
                        if field not in sample or sample[field] is None:
                            missing_fields.append(field)
                    
                    if missing_fields:
                        print(f"⚠ Warning: Missing fields: {missing_fields}")
                    else:
                        print("✓ All expected fields present")
                        success_count += 1
                
            else:
                print(f"✗ Error: HTTP {response.status_code}")
                print(f"Response: {response.text}")
                
        except Exception as e:
            print(f"✗ Exception: {e}")
        
        print()
    
    print(f"=== Microservice Test Complete: {success_count}/{total_tests} successful ===")
    return success_count == total_tests

def test_microservice_with_large_file():
    """Test with a larger log file to check performance"""
    print("\n=== Testing Large Log File Performance ===")
    
    # Generate a larger test file
    large_content = []
    for i in range(100):
        large_content.append(f"2024-06-01 12:{i:02d}:00 INFO Log entry {i} - System operation completed")
        if i % 10 == 0:
            large_content.append(f"2024-06-01 12:{i:02d}:05 WARN Warning message {i} - Resource usage high")
        if i % 20 == 0:
            large_content.append(f"2024-06-01 12:{i:02d}:10 ERROR Error message {i} - Operation failed")
    
    content = "\n".join(large_content)
    
    try:
        with tempfile.NamedTemporaryFile(mode='w', suffix='.log', delete=False) as tmp_file:
            tmp_file.write(content)
            tmp_file_path = tmp_file.name
        
        start_time = time.time()
        
        with open(tmp_file_path, 'rb') as f:
            files = {'file': ('large_test.log', f, 'text/plain')}
            response = requests.post("http://localhost:8001/parse", files=files)
        
        end_time = time.time()
        processing_time = end_time - start_time
        
        os.unlink(tmp_file_path)
        
        if response.status_code == 200:
            result = response.json()
            print(f"✓ Success: Parsed {len(result)} entries in {processing_time:.2f} seconds")
            print(f"  Processing rate: {len(result)/processing_time:.1f} entries/second")
            return True
        else:
            print(f"✗ Error: HTTP {response.status_code}")
            print(f"Response: {response.text}")
            return False
            
    except Exception as e:
        print(f"✗ Exception: {e}")
        return False

def create_realistic_log_scenarios():
    """Create realistic log scenarios for testing"""
    scenarios = {
        "production_web_server": {
            "name": "Production Web Server Logs",
            "content": """192.168.1.100 - - [01/Jun/2024:12:00:00 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"
192.168.1.101 - - [01/Jun/2024:12:00:01 +0000] "POST /api/login HTTP/1.1" 401 567 "https://example.com/login" "Mozilla/5.0"
192.168.1.102 - - [01/Jun/2024:12:00:02 +0000] "GET /api/data HTTP/1.1" 500 890 "https://example.com/data" "Mozilla/5.0"
192.168.1.103 - - [01/Jun/2024:12:00:03 +0000] "PUT /api/update HTTP/1.1" 200 234 "https://example.com/update" "Mozilla/5.0"
192.168.1.104 - - [01/Jun/2024:12:00:04 +0000] "DELETE /api/delete HTTP/1.1" 404 123 "https://example.com/delete" "Mozilla/5.0"
"""
        },
        "kubernetes_cluster": {
            "name": "Kubernetes Cluster Logs",
            "content": """2024-06-01T12:00:00.123Z pod1 container1 INFO Pod initialization started
2024-06-01T12:00:01.456Z pod2 container2 WARN Resource limits approaching: CPU 85%
2024-06-01T12:00:02.789Z pod3 container3 ERROR Service discovery failed: timeout
2024-06-01T12:00:03.012Z pod1 container1 INFO Health check passed
2024-06-01T12:00:04.345Z pod2 container2 DEBUG ConfigMap updated: version=1.2.3
2024-06-01T12:00:05.678Z pod3 container3 ERROR CrashLoopBackOff: container failed to start
"""
        },
        "application_logs": {
            "name": "Application Logs (Mixed JSON/Structured)",
            "content": """{"timestamp":"2024-06-01T12:00:00Z","level":"INFO","message":"Application startup","version":"1.2.3","environment":"production"}
2024-06-01 12:00:01 [INFO] Database connection established - pool_size=20
{"timestamp":"2024-06-01T12:00:02Z","level":"WARN","message":"High memory usage","memory_percent":85,"heap_used":"512MB"}
2024-06-01 12:00:03 [ERROR] Failed to process user request: user_id=12345, error=timeout
{"timestamp":"2024-06-01T12:00:04Z","level":"INFO","message":"Cache miss","key":"user_profile_67890","cache_hit_rate":0.85}
2024-06-01 12:00:05 [DEBUG] SQL query executed: SELECT * FROM orders WHERE user_id = ?
"""
        },
        "security_logs": {
            "name": "Security Event Logs",
            "content": """2024-06-01T12:00:00.123Z SECURITY WARN Failed login attempt from 192.168.1.100 - user=admin
2024-06-01T12:00:01.456Z SECURITY INFO Successful authentication: user=john, ip=192.168.1.101
2024-06-01T12:00:02.789Z SECURITY ERROR Unauthorized access attempt: resource=/admin, user=guest
2024-06-01T12:00:03.012Z SECURITY INFO Password changed: user=john, ip=192.168.1.101
2024-06-01T12:00:04.345Z SECURITY WARN Multiple failed attempts: user=guest, ip=192.168.1.102
2024-06-01T12:00:05.678Z SECURITY ERROR Brute force attack detected: ip=192.168.1.103, attempts=50
"""
        },
        "system_logs": {
            "name": "System Logs (Various Formats)",
            "content": """2024-06-01 12:00:00 INFO System startup completed - uptime=0s
2024-06-01 12:00:01 WARN High memory usage detected: 85% (8.5GB/10GB)
2024-06-01 12:00:02 ERROR Database connection failed: timeout after 30s
2024-06-01 12:00:03 INFO User login successful: user=john, session_id=abc123
2024-06-01 12:00:04 DEBUG Processing request: id=12345, method=GET, path=/api/data
2024-06-01 12:00:05 FATAL Critical system failure: disk full on /var/log
"""
        }
    }
    return scenarios

def analyze_parsing_results(results: List[Dict], scenario_name: str):
    """Analyze the parsing results and provide insights"""
    print(f"\n📊 Analysis for {scenario_name}:")
    print("-" * 60)
    
    if not results:
        print("❌ No results to analyze")
        return
    
    # Basic statistics
    total_entries = len(results)
    timestamp_count = sum(1 for r in results if r.get('timestamp'))
    level_count = sum(1 for r in results if r.get('level'))
    metadata_count = sum(1 for r in results if r.get('metadata'))
    
    print(f"📈 Basic Statistics:")
    print(f"  • Total entries: {total_entries}")
    print(f"  • Entries with timestamp: {timestamp_count} ({timestamp_count/total_entries*100:.1f}%)")
    print(f"  • Entries with level: {level_count} ({level_count/total_entries*100:.1f}%)")
    print(f"  • Entries with metadata: {metadata_count} ({metadata_count/total_entries*100:.1f}%)")
    
    # Level distribution
    level_dist = {}
    for result in results:
        level = result.get('level', 'UNKNOWN')
        level_dist[level] = level_dist.get(level, 0) + 1
    
    print(f"\n📊 Level Distribution:")
    for level, count in sorted(level_dist.items()):
        percentage = count / total_entries * 100
        print(f"  • {level}: {count} ({percentage:.1f}%)")
    
    # Metadata analysis
    all_metadata_keys = set()
    for result in results:
        if result.get('metadata'):
            all_metadata_keys.update(result['metadata'].keys())
    
    if all_metadata_keys:
        print(f"\n🔍 Metadata Fields Found:")
        for key in sorted(all_metadata_keys):
            count = sum(1 for r in results if r.get('metadata', {}).get(key))
            print(f"  • {key}: {count} entries")
    
    # Sample entries
    print(f"\n📝 Sample Entries:")
    for i, result in enumerate(results[:3], 1):
        print(f"  {i}. {result.get('message', 'No message')[:80]}...")
        if result.get('level'):
            print(f"     Level: {result['level']}")
        if result.get('timestamp'):
            print(f"     Time: {result['timestamp']}")

def test_real_world_scenarios():
    """Test with realistic log scenarios"""
    print("\n🚀 Enhanced ML Parser - Real-World Integration Test")
    print("=" * 70)
    
    scenarios = create_realistic_log_scenarios()
    base_url = "http://localhost:8001"
    
    total_start_time = time.time()
    success_count = 0
    total_tests = len(scenarios)
    
    for scenario_key, scenario in scenarios.items():
        print(f"\n🔬 Testing: {scenario['name']}")
        print("=" * 50)
        
        try:
            # Create temporary file
            with tempfile.NamedTemporaryFile(mode='w', suffix='.log', delete=False) as tmp_file:
                tmp_file.write(scenario['content'])
                tmp_file_path = tmp_file.name
            
            # Measure parsing time
            start_time = time.time()
            
            # Send to microservice
            with open(tmp_file_path, 'rb') as f:
                files = {'file': (f'{scenario_key}.log', f, 'text/plain')}
                response = requests.post(f"{base_url}/parse", files=files)
            
            end_time = time.time()
            processing_time = end_time - start_time
            
            # Clean up
            os.unlink(tmp_file_path)
            
            if response.status_code == 200:
                results = response.json()
                print(f"✅ Success: Parsed {len(results)} entries in {processing_time:.3f}s")
                print(f"   Performance: {len(results)/processing_time:.1f} entries/second")
                
                # Analyze results
                analyze_parsing_results(results, scenario['name'])
                success_count += 1
                
            else:
                print(f"❌ Error: HTTP {response.status_code}")
                print(f"   Response: {response.text}")
                
        except Exception as e:
            print(f"❌ Exception: {e}")
    
    total_time = time.time() - total_start_time
    print(f"\n🎯 Test Summary:")
    print(f"   Total test time: {total_time:.2f} seconds")
    print(f"   Scenarios tested: {total_tests}")
    print(f"   Successful: {success_count}/{total_tests}")
    
    return success_count == total_tests

def test_performance_with_large_dataset():
    """Test performance with a large dataset"""
    print(f"\n⚡ Performance Test with Large Dataset")
    print("=" * 50)
    
    # Generate a large realistic dataset
    large_content = []
    log_types = [
        "INFO System operation completed successfully",
        "WARN Resource usage high: CPU 85%, Memory 90%",
        "ERROR Database connection timeout after 30 seconds",
        "DEBUG Processing request ID: {id}, Method: GET, Path: /api/data",
        "INFO User authentication successful: user={user}, session={session}",
        "ERROR File not found: /var/log/app.log",
        "WARN High disk usage detected: 95% on /var",
        "INFO Cache miss for key: user_profile_{id}",
        "ERROR Network timeout: connection to database failed",
        "DEBUG SQL query executed: SELECT * FROM users WHERE id = {id}"
    ]
    
    for i in range(500):  # 500 log entries
        log_type = log_types[i % len(log_types)]
        timestamp = f"2024-06-01 12:{i//60:02d}:{i%60:02d}"
        content = f"{timestamp} {log_type}".format(id=i, user=f"user{i}", session=f"session{i}")
        large_content.append(content)
    
    content = "\n".join(large_content)
    
    try:
        with tempfile.NamedTemporaryFile(mode='w', suffix='.log', delete=False) as tmp_file:
            tmp_file.write(content)
            tmp_file_path = tmp_file.name
        
        start_time = time.time()
        
        with open(tmp_file_path, 'rb') as f:
            files = {'file': ('large_performance_test.log', f, 'text/plain')}
            response = requests.post("http://localhost:8001/parse", files=files)
        
        end_time = time.time()
        processing_time = end_time - start_time
        
        os.unlink(tmp_file_path)
        
        if response.status_code == 200:
            results = response.json()
            print(f"✅ Large Dataset Results:")
            print(f"   Entries processed: {len(results)}")
            print(f"   Processing time: {processing_time:.3f} seconds")
            print(f"   Throughput: {len(results)/processing_time:.1f} entries/second")
            print(f"   Average time per entry: {processing_time/len(results)*1000:.2f} ms")
            
            # Memory efficiency (rough estimate)
            avg_entry_size = sum(len(str(r)) for r in results) / len(results)
            print(f"   Average entry size: {avg_entry_size:.0f} characters")
            return True
            
        else:
            print(f"❌ Error: HTTP {response.status_code}")
            print(f"   Response: {response.text}")
            return False
            
    except Exception as e:
        print(f"❌ Exception: {e}")
        return False

def test_health_check():
    """Test if the microservice is running"""
    print("\n=== Health Check ===")
    try:
        response = requests.get("http://localhost:8001/health", timeout=5)
        if response.status_code == 200:
            print("✅ Microservice is running and healthy")
            return True
        else:
            print(f"❌ Microservice returned status {response.status_code}")
            return False
    except Exception as e:
        print(f"❌ Cannot connect to microservice: {e}")
        return False

def main():
    """Main function to run tests based on command line arguments"""
    parser = argparse.ArgumentParser(description="Enhanced ML Logparser - Comprehensive Test Suite")
    parser.add_argument("--ml-only", action="store_true", help="Run only ML parser tests")
    parser.add_argument("--microservice", action="store_true", help="Run only microservice tests")
    parser.add_argument("--real-world", action="store_true", help="Run only real-world scenario tests")
    parser.add_argument("--performance", action="store_true", help="Run only performance tests")
    parser.add_argument("--health", action="store_true", help="Run only health check tests")
    parser.add_argument("--list", action="store_true", help="List all available test cases")
    parser.add_argument("--add-test", type=str, help="Add a new test case (provide JSON file path)")
    parser.add_argument("--all", action="store_true", help="Run all tests")
    
    args = parser.parse_args()
    
    print("🧪 Enhanced ML Logparser - Comprehensive Test Suite")
    print("=" * 60)
    print(f"Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print()
    
    # Handle special commands
    if args.list:
        list_test_cases()
        return
    
    if args.add_test:
        try:
            import json
            with open(args.add_test, 'r') as f:
                new_test = json.load(f)
            category = new_test.get('category', 'ml_parser_test_cases')
            add_test_case(category, new_test)
        except Exception as e:
            print(f"❌ Error adding test case: {e}")
        return
    
    # Run tests based on arguments
    results = {}
    
    if args.ml_only or args.all:
        results["ML Only"] = test_ml_only_analysis()
    
    if args.microservice or args.all:
        results["Microservice"] = test_microservice_parsing()
    
    if args.real_world or args.all:
        results["Real World"] = test_real_world_scenarios()
    
    if args.performance or args.all:
        results["Performance"] = test_performance_with_large_dataset()
    
    if args.health or args.all:
        results["Health Check"] = test_health_check()
    
    # If no specific test type specified, run ML-only by default
    if not any([args.ml_only, args.microservice, args.real_world, args.performance, args.health, args.all]):
        results["ML Only"] = test_ml_only_analysis()
    
    # Print summary
    print("\n" + "=" * 60)
    print("📊 TEST SUMMARY")
    print("=" * 60)
    
    passed = 0
    total = len(results)
    
    for test_name, result in results.items():
        status = "✅ PASS" if result else "❌ FAIL"
        print(f"{test_name}: {status}")
        if result:
            passed += 1
    
    print(f"\nOverall: {passed}/{total} tests passed")
    
    if passed < total:
        print("⚠️  Some tests failed. Please check the output above.")
    else:
        print("🎉 All tests passed!")
    
    print(f"\nCompleted at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")

if __name__ == "__main__":
    main() 