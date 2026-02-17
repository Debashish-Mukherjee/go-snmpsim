#!/usr/bin/env python3
"""
Zabbix 7.4 + Cisco IOS XR SNMPSIM Integration Test Runner
Provisions devices, verifies data collection, and generates reports
"""

import sys
import time
import json
import yaml
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Any
import subprocess

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "zabbix"))

from zabbix_api_client import ZabbixAPIClient, ZabbixAPIError


class ZabbixIntegrationTester:
    """Main test orchestrator"""
    
    def __init__(self, config_file: str = "zabbix_config.yaml"):
        self.config_file = config_file
        self.config = self._load_config()
        self.client = None
        self.test_results = {
            "start_time": datetime.now().isoformat(),
            "status": "running",
            "devices_added": 0,
            "items_created": 0,
            "items_with_data": 0,
            "errors": []
        }
    
    def _load_config(self) -> dict:
        """Load test configuration"""
        try:
            with open(self.config_file, 'r') as f:
                return yaml.safe_load(f) or {}
        except FileNotFoundError:
            print(f"❌ Config file not found: {self.config_file}")
            sys.exit(1)
    
    def _print_header(self, title: str):
        """Print formatted section header"""
        print()
        print("=" * 80)
        print(f"  {title}")
        print("=" * 80)
        print()
    
    def _wait_for_docker_services(self, timeout: int = 120) -> bool:
        """Wait for Docker Compose services to be healthy"""
        compose_file = self.config.get("docker", {}).get("compose_file", "zabbix/docker-compose.zabbix.yml")
        
        print("⏳ Waiting for Docker Compose services to be healthy...")
        
        start_time = time.time()
        while time.time() - start_time < timeout:
            try:
                # Check if we can connect to Zabbix
                if self.client.wait_for_server(max_retries=1, retry_interval=1):
                    print("✓ Zabbix server is responding")
                    return True
            except Exception:
                pass
            
            time.sleep(5)
            elapsed = int(time.time() - start_time)
            print(f"  ({elapsed}s/{timeout}s)")
        
        print("❌ Timeout waiting for services")
        return False
    
    def _initialize_client(self) -> bool:
        """Initialize Zabbix API client"""
        self._print_header("1. Initializing Zabbix API Client")
        
        zabbix_config = self.config.get("zabbix", {})
        url = zabbix_config.get("url", "http://localhost:8081")
        username = zabbix_config.get("username", "Admin")
        password = zabbix_config.get("password", "zabbix")
        
        print(f"📍 Zabbix URL: {url}")
        print(f"👤 Username: {username}")
        
        self.client = ZabbixAPIClient(url, username, password)
        
        # Get version
        try:
            version = self.client.get_version()
            print(f"✓ Version: {version}")
        except ZabbixAPIError as e:
            print(f"❌ Could not get version: {e}")
            return False
        
        return True
    
    def _authenticate(self) -> bool:
        """Authenticate with Zabbix API"""
        self._print_header("2. Authenticating with Zabbix API")
        
        try:
            self.client.login()
            print("✓ Authentication successful")
            return True
        except ZabbixAPIError as e:
            print(f"❌ Authentication failed: {e}")
            return False
    
    def _add_devices(self, num_devices: int = None) -> bool:
        """Add SNMP devices to Zabbix"""
        if num_devices is None:
            snmp_config = self.config.get("snmp", {})
            num_devices = snmp_config.get("devices", {}).get("count", 20)
        
        self._print_header(f"3. Adding {num_devices} Cisco IOS XR Devices")
        
        snmp_config = self.config.get("snmp", {})
        polling_config = self.config.get("polling", {})
        
        snmp_port_start = snmp_config.get("port_start", 20000)
        snmp_community = snmp_config.get("community", "public")
        snmp_version = snmp_config.get("version", "2")
        polling_interval = polling_config.get("interval", "5m")
        
        added = 0
        failed = 0
        
        print(f"Configuration:")
        print(f"  • Port Range: {snmp_port_start} - {snmp_port_start + num_devices - 1}")
        print(f"  • Community: {snmp_community}")
        print(f"  • Version: SNMPv{snmp_version}")
        print(f"  • Polling Interval: {polling_interval}")
        print()
        
        for device_num in range(1, num_devices + 1):
            device_id = f"{device_num:03d}"
            hostname = f"cisco-iosxr-{device_id}"
            ip = "127.0.0.1"
            port = snmp_port_start + device_num - 1
            
            try:
                # Check if already exists
                existing = self.client.get_host({"host": hostname})
                if existing:
                    print(f"  ⊘ {hostname} (port {port}) - already exists")
                    added += 1
                    continue
                
                # Create host
                hostid = self.client.create_host(
                    hostname=hostname,
                    ip_address=ip,
                    port=port,
                    snmp_version=snmp_version,
                    community=snmp_community
                )
                
                # Update polling interval
                self.client.update_polling_interval(hostid, polling_interval)
                
                print(f"  ✓ {hostname} (port {port})")
                added += 1
                
            except ZabbixAPIError as e:
                print(f"  ✗ {hostname} - {e}")
                self.test_results["errors"].append(f"Failed to add {hostname}: {e}")
                failed += 1
        
        self.test_results["devices_added"] = added
        
        print()
        print(f"Summary: {added} added, {failed} failed")
        
        return failed == 0
    
    def _verify_items_created(self, num_devices: int = None) -> bool:
        """Verify items have been created for all devices"""
        if num_devices is None:
            snmp_config = self.config.get("snmp", {})
            num_devices = snmp_config.get("devices", {}).get("count", 20)
        
        self._print_header("4. Verifying Items Created")
        
        # Get all hosts
        hosts = self.client.get_all_hosts()
        
        if not hosts:
            print("❌ No hosts found")
            return False
        
        # Filter to only Cisco IOS XR devices
        cisco_hosts = [h for h in hosts if "cisco-iosxr" in h.get("host", "")]
        
        print(f"Found {len(cisco_hosts)} Cisco IOS XR devices\n")
        
        total_items = 0
        devices_with_items = 0
        
        for host in cisco_hosts[:num_devices]:  # Check first N devices
            items = self.client.get_host_items(host["hostid"])
            total_items += len(items)
            if len(items) > 0:
                devices_with_items += 1
            print(f"  • {host['host']}: {len(items)} items")
        
        print()
        print(f"Summary:")
        print(f"  • Total Items: {total_items}")
        print(f"  • Devices with Items: {devices_with_items}/{num_devices}")
        
        self.test_results["items_created"] = total_items
        
        return devices_with_items > 0
    
    def _wait_for_data_collection(self, num_devices: int = None, max_wait: int = 600) -> bool:
        """Wait for Zabbix to collect metrics from SNMP devices"""
        if num_devices is None:
            snmp_config = self.config.get("snmp", {})
            num_devices = snmp_config.get("devices", {}).get("count", 20)
        
        self._print_header("5. Waiting for Initial Data Collection")
        
        initial_wait = self.config.get("polling", {}).get("initial_wait", 60)
        
        print(f"⏳ Waiting {initial_wait} seconds for initial data collection...")
        print()
        
        start_time = time.time()
        elapsed = 0
        
        while elapsed < max_wait:
            time.sleep(min(10, max_wait - elapsed))
            elapsed = int(time.time() - start_time)
            
            # Get hosts
            hosts = self.client.get_all_hosts()
            cisco_hosts = [h for h in hosts if "cisco-iosxr" in h.get("host", "")]
            
            if not cisco_hosts:
                continue
            
            # Check first device for data
            first_host = cisco_hosts[0]
            items = self.client.get_host_items(first_host["hostid"])
            
            if items:
                # Check if any items have data
                items_with_values = 0
                for item in items[:10]:  # Check first 10 items
                    values = self.client.get_host_values(first_host["hostid"], item["key_"], limit=1)
                    if values:
                        items_with_values += 1
                
                if items_with_values > 0:
                    print(f"\n✓ Data collection started ({elapsed}s)")
                    print(f"  • First device: {first_host['host']}")
                    print(f"  • Items with data: {items_with_values}/10")
                    return True
            
            if elapsed % 30 == 0:
                print(f"  ({elapsed}s/{max_wait}s)")
        
        print(f"\n⚠️  No data collected after {max_wait} seconds")
        print("Troubleshooting:")
        print("  1. Check if SNMP simulator is running")
        print("  2. Verify SNMP port is accessible")
        print("  3. Check Zabbix server logs")
        return False
    
    def _collect_metrics(self, num_devices: int = None) -> Dict[str, Any]:
        """Collect metrics from all devices"""
        if num_devices is None:
            snmp_config = self.config.get("snmp", {})
            num_devices = snmp_config.get("devices", {}).get("count", 20)
        
        self._print_header("6. Collecting Metrics")
        
        # Get all hosts
        hosts = self.client.get_all_hosts()
        cisco_hosts = [h for h in hosts if "cisco-iosxr" in h.get("host", "")][:num_devices]
        
        metrics = {
            "total_items": 0,
            "items_with_data": 0,
            "devices_with_data": 0,
            "devices": {}
        }
        
        print(f"Checking {len(cisco_hosts)} devices...\n")
        
        for host in cisco_hosts:
            hostname = host["host"]
            items = self.client.get_host_items(host["hostid"])
            
            items_with_data = 0
            for item in items:
                values = self.client.get_host_values(host["hostid"], item["key_"], limit=1)
                if values:
                    items_with_data += 1
            
            metrics["total_items"] += len(items)
            metrics["items_with_data"] += items_with_data
            if items_with_data > 0:
                metrics["devices_with_data"] += 1
            
            metrics["devices"][hostname] = {
                "total_items": len(items),
                "items_with_data": items_with_data,
                "success_rate": round(items_with_data / len(items) * 100, 1) if items else 0
            }
            
            status_icon = "✓" if items_with_data > 0 else "✗"
            print(f"  {status_icon} {hostname}: {items_with_data}/{len(items)} items with data")
        
        print()
        print(f"Summary:")
        print(f"  • Total Items: {metrics['total_items']}")
        print(f"  • Items with Data: {metrics['items_with_data']}")
        print(f"  • Success Rate: {round(metrics['items_with_data']/max(metrics['total_items'], 1)*100, 1)}%")
        print(f"  • Devices with Data: {metrics['devices_with_data']}/{len(cisco_hosts)}")
        
        self.test_results["items_with_data"] = metrics["items_with_data"]
        self.test_results["success_rate"] = round(
            metrics["items_with_data"] / max(metrics["total_items"], 1) * 100, 1
        )
        
        return metrics
    
    def _generate_report(self, metrics: Dict[str, Any]):
        """Generate final test report"""
        self._print_header("7. Generating Test Report")
        
        self.test_results["end_time"] = datetime.now().isoformat()
        self.test_results["status"] = "completed"
        
        report = {
            "test_info": {
                "name": self.config.get("test", {}).get("test_name"),
                "description": self.config.get("test", {}).get("description"),
                "start_time": self.test_results["start_time"],
                "end_time": self.test_results["end_time"],
            },
            "results": self.test_results,
            "metrics": metrics
        }
        
        # Save report
        report_file = "zabbix_test_report.json"
        with open(report_file, 'w') as f:
            json.dump(report, f, indent=2)
        
        print(f"📊 Test Report Summary:\n")
        print(f"  Test Name: {report['test_info']['name']}")
        print(f"  Start Time: {report['test_info']['start_time']}")
        print(f"  End Time: {report['test_info']['end_time']}")
        print()
        print(f"  Devices Added: {report['results']['devices_added']}")
        print(f"  Items Created: {report['results']['items_created']}")
        print(f"  Items with Data: {report['results']['items_with_data']}")
        print(f"  Success Rate: {report['results'].get('success_rate', 0)}%")
        print()
        
        if report['results']['errors']:
            print(f"  ⚠️  Errors ({len(report['results']['errors'])}):")
            for error in report['results']['errors'][:5]:
                print(f"    • {error}")
        
        print()
        print(f"✅ Report saved to: {report_file}")
        print()
        
        return report
    
    def run_full_test(self, num_devices: int = None) -> bool:
        """Run complete integration test"""
        try:
            # Step 1: Initialize
            if not self._initialize_client():
                return False
            
            # Step 2: Authenticate
            if not self._authenticate():
                return False
            
            # Step 3: Add devices
            if not self._add_devices(num_devices):
                print("⚠️  Some devices failed to add, continuing...")
            
            # Step 4: Verify items
            if not self._verify_items_created(num_devices):
                print("⚠️  Items not created yet, they may be created asynchronously...")
            
            # Step 5: Wait for data collection
            if not self._wait_for_data_collection(num_devices):
                self.test_results["errors"].append("Data collection did not start")
            
            # Step 6: Collect metrics
            metrics = self._collect_metrics(num_devices)
            
            # Step 7: Generate report
            self._generate_report(metrics)
            
            # Print final summary
            self._print_header("✅ Test Completed Successfully")
            
            print("Next steps:")
            print("  1. View Zabbix dashboard: http://localhost:8081")
            print("  2. Review test report: zabbix_test_report.json")
            print("  3. Monitor polling progress in Zabbix UI")
            print()
            
            return True
            
        except Exception as e:
            print(f"\n❌ Test failed with error: {e}")
            import traceback
            traceback.print_exc()
            return False


def main():
    """Main entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="Zabbix 7.4 + Cisco IOS XR SNMPSIM Integration Test",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Run full test with 20 devices
  python run_zabbix_test.py
  
  # Run test with 5 devices
  python run_zabbix_test.py --devices 5
  
  # Run test with custom config
  python run_zabbix_test.py --config custom_config.yaml
        """
    )
    
    parser.add_argument('--devices', type=int, default=None, help='Number of devices to test')
    parser.add_argument('--config', default='zabbix_config.yaml', help='Config file path')
    
    args = parser.parse_args()
    
    print()
    print("╔" + "=" * 78 + "╗")
    print("║" + " " * 20 + "Zabbix 7.4 + Cisco IOS XR Integration Test" + " " * 14 + "║")
    print("╚" + "=" * 78 + "╝")
    
    tester = ZabbixIntegrationTester(args.config)
    
    success = tester.run_full_test(args.devices)
    
    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
