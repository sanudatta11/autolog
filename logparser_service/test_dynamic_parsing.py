#!/usr/bin/env python3
"""
Test script to demonstrate dynamic parsing capability
"""

import requests
import json

def test_dynamic_parsing():
    """Test dynamic parsing with systemd/rclone logs"""
    
    # Test log lines that were causing preprocessing failures
    test_lines = [
        "2025-07-04T01:56:15.212666+02:00 v2202503259448321762 rclone[1514610]: INFO  : vfs cache: cleaned: objects 0 (was 0) in use 0, to upload 0, uploading 0, total size 0 (was 0)",
        "2025-07-04T01:56:27.211485+02:00 v2202503259448321762 systemd[1]: amptasks.service: Found left-over process 1444099 (tmux: server) in control group while starting unit. Ignoring.",
        "2025-07-04T01:56:27.211640+02:00 v2202503259448321762 systemd[1]: This usually indicates unclean termination of a previous run, or service implementation deficiencies.",
        "2025-07-04T01:56:27.211676+02:00 v2202503259448321762 systemd[1]: amptasks.service: Found left-over process 1444100 (AMP_Linux_x86_6) in control group while starting unit. Ignoring.",
        "2025-07-04T01:56:27.230960+02:00 v2202503259448321762 systemd[1]: Starting amptasks.service - AMP Instance Manager Pending Tasks...",
        "2025-07-04T01:56:27.752270+02:00 v2202503259448321762 ampinstmgr[3770073]: [Info/1] AMP Instance Manager v2.6.2 built 01/06/2025 14:19",
        "2025-07-04T01:56:27.752615+02:00 v2202503259448321762 ampinstmgr[3770073]: [Info/1] Stream: Mainline / Release - built by CUBECODERS/buildbot on CCL-DEV",
        "2025-07-04T01:56:27.924044+02:00 v2202503259448321762 systemd[1]: amptasks.service: Deactivated successfully.",
        "2025-07-04T01:56:27.924170+02:00 v2202503259448321762 systemd[1]: amptasks.service: Unit process 1444099 (tmux: server) remains running after unit stopped.",
        "2025-07-04T01:56:27.924213+02:00 v2202503259448321762 systemd[1]: amptasks.service: Unit process 1444100 (AMP_Linux_x86_6) remains running after unit stopped.",
        "2025-07-04T01:56:27.924339+02:00 v2202503259448321762 systemd[1]: Finished amptasks.service - AMP Instance Manager Pending Tasks.",
        "2025-07-04T01:57:15.213096+02:00 v2202503259448321762 rclone[1514610]: INFO  : vfs cache: cleaned: objects 0 (was 0) in use 0, to upload 0, uploading 0, total size 0 (was 0)",
        "2025-07-04T01:57:28.410828+02:00 v2202503259448321762 systemd[1]: amptasks.service: Found left-over process 1444099 (tmux: server) in control group while starting unit. Ignoring.",
        "2025-07-04T01:57:28.410962+02:00 v2202503259448321762 systemd[1]: This usually indicates unclean termination of a previous run, or service implementation deficiencies.",
        "2025-07-04T01:57:28.411000+02:00 v2202503259448321762 systemd[1]: amptasks.service: Found left-over process 1444100 (AMP_Linux_x86_6) in control group while starting unit. Ignoring.",
        "2025-07-04T01:57:28.411036+02:00 v2202503259448321762 systemd[1]: This usually indicates unclean termination of a previous run, or service implementation deficiencies.",
        "2025-07-04T01:57:28.443119+02:00 v2202503259448321762 systemd[1]: Starting amptasks.service - AMP Instance Manager Pending Tasks...",
        "2025-07-04T01:57:28.973175+02:00 v2202503259448321762 ampinstmgr[3770130]: [Info/1] AMP Instance Manager v2.6.2 built 01/06/2025 14:19"
    ]
    
    print("Testing Dynamic Parsing with Systemd/Rclone Logs")
    print("=" * 60)
    print(f"Total test lines: {len(test_lines)}")
    print()
    
    # Test regular parsing
    print("1. Testing regular parsing endpoint...")
    try:
        response = requests.post(
            "http://localhost:8000/parse",
            files={"file": ("test.log", "\n".join(test_lines))},
            timeout=30
        )
        
        if response.status_code == 200:
            regular_entries = response.json()
            print(f"   ✓ Regular parsing successful: {len(regular_entries)} entries parsed")
            
            # Show sample entries
            for i, entry in enumerate(regular_entries[:3]):
                print(f"   Entry {i+1}:")
                print(f"     Timestamp: {entry.get('timestamp', 'N/A')}")
                print(f"     Level: {entry.get('level', 'N/A')}")
                print(f"     Message: {entry.get('message', 'N/A')[:100]}...")
                print(f"     Metadata: {entry.get('metadata', {})}")
                print()
        else:
            print(f"   ✗ Regular parsing failed: {response.status_code} - {response.text}")
            
    except Exception as e:
        print(f"   ✗ Regular parsing error: {str(e)}")
    
    # Test dynamic parsing
    print("2. Testing dynamic parsing endpoint...")
    try:
        response = requests.post(
            "http://localhost:8000/parse-dynamic",
            json={"lines": test_lines},
            timeout=30
        )
        
        if response.status_code == 200:
            dynamic_entries = response.json()
            print(f"   ✓ Dynamic parsing successful: {len(dynamic_entries)} entries parsed")
            
            # Show sample entries
            for i, entry in enumerate(dynamic_entries[:3]):
                print(f"   Entry {i+1}:")
                print(f"     Timestamp: {entry.get('timestamp', 'N/A')}")
                print(f"     Level: {entry.get('level', 'N/A')}")
                print(f"     Message: {entry.get('message', 'N/A')[:100]}...")
                print(f"     Metadata: {entry.get('metadata', {})}")
                print()
        else:
            print(f"   ✗ Dynamic parsing failed: {response.status_code} - {response.text}")
            
    except Exception as e:
        print(f"   ✗ Dynamic parsing error: {str(e)}")
    
    # Test systemd-specific parsing
    print("3. Testing systemd/rclone specific parsing...")
    try:
        # Test the systemd pattern directly
        import re
        
        pattern = r'^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2}))\s+(\S+)\s+(\w+)\[(\d+)\]:\s*(?:(INFO|DEBUG|WARN|WARNING|ERROR|FATAL|CRITICAL)[:\s-]*)?(.*)$'
        
        successful_matches = 0
        for line in test_lines:
            match = re.match(pattern, line)
            if match:
                successful_matches += 1
        
        print(f"   ✓ Systemd pattern matching: {successful_matches}/{len(test_lines)} lines matched ({successful_matches/len(test_lines)*100:.1f}%)")
        
        # Show a sample match
        for line in test_lines[:1]:
            match = re.match(pattern, line)
            if match:
                timestamp, hostname, process, pid, level, message = match.groups()
                print(f"   Sample match:")
                print(f"     Timestamp: {timestamp}")
                print(f"     Hostname: {hostname}")
                print(f"     Process: {process}")
                print(f"     PID: {pid}")
                print(f"     Level: {level or 'INFO (default)'}")
                print(f"     Message: {message.strip()}")
                print()
                
    except Exception as e:
        print(f"   ✗ Systemd pattern testing error: {str(e)}")
    
    print("=" * 60)
    print("Dynamic Parsing Test Complete!")

if __name__ == "__main__":
    test_dynamic_parsing() 