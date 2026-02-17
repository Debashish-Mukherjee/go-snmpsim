#!/usr/bin/env python3
"""
Device Management CLI - Add/delete/list/configure Cisco IOS XR devices in Zabbix
"""

import argparse
import sys
import time
import yaml
from pathlib import Path
from zabbix_api_client import ZabbixAPIClient, ZabbixAPIError


def load_config(config_file: str = "zabbix_config.yaml") -> dict:
    """Load configuration from YAML file"""
    try:
        with open(config_file, 'r') as f:
            return yaml.safe_load(f) or {}
    except FileNotFoundError:
        print(f"⚠️  Config file not found: {config_file}")
        return {}


def get_client(zabbix_url: str = None, config_file: str = "zabbix_config.yaml") -> ZabbixAPIClient:
    """Initialize Zabbix API client with config"""
    config = load_config(config_file)
    
    url = zabbix_url or config.get("zabbix_url", "http://localhost:8081")
    username = config.get("zabbix_username", "Admin")
    password = config.get("zabbix_password", "zabbix")
    
    client = ZabbixAPIClient(url, username, password)
    return client


def cmd_add_devices(args):
    """Add N devices to Zabbix"""
    config = load_config()
    
    client = get_client()
    
    print()
    print(f"🔗 Connecting to Zabbix at {client.url}...")
    print(f"   Version: {client.get_version()}")
    
    # Wait for server to be ready
    print()
    print("⏳ Waiting for Zabbix server to be ready...")
    if not client.wait_for_server():
        print("❌ Zabbix server not responding. Make sure it's running:")
        print("   docker-compose -f zabbix/docker-compose.zabbix.yml up -d")
        return 1
    
    print("✓ Zabbix server ready!\n")
    
    # Authenticate
    print("🔑 Authenticating...")
    try:
        client.login()
        print("✓ Authentication successful!\n")
    except ZabbixAPIError as e:
        print(f"❌ Authentication failed: {e}")
        print("\nDefault credentials:")
        print("  Username: Admin")
        print("  Password: zabbix")
        print("\nOr update them in zabbix_config.yaml")
        return 1
    
    # Get SNMP simulator config
    snmp_port_start = config.get("snmp_port_start", 20000)
    snmp_community = config.get("snmp_community", "public")
    snmp_version = config.get("snmp_version", "2")
    polling_interval = config.get("polling_interval", "5m")
    
    # Add devices
    num_devices = args.count
    print(f"📦 Adding {num_devices} Cisco IOS XR devices to Zabbix...\n")
    
    added_hosts = []
    failed_hosts = []
    
    for device_num in range(1, num_devices + 1):
        device_id = f"{device_num:03d}"
        hostname = f"cisco-iosxr-{device_id}"
        ip = "127.0.0.1"  # localhost for simulator
        port = snmp_port_start + device_num - 1
        
        try:
            # Check if host already exists
            existing = client.get_host({"host": hostname})
            if existing:
                print(f"⊘ Device {device_num:2d}: {hostname} (port {port}) - already exists")
                added_hosts.append((hostname, port))
                continue
            
            # Create host
            hostid = client.create_host(
                hostname=hostname,
                ip_address=ip,
                port=port,
                snmp_version=snmp_version,
                community=snmp_community
            )
            
            # Update polling interval
            client.update_polling_interval(hostid, polling_interval)
            
            print(f"✓ Device {device_num:2d}: {hostname} (port {port}) [ID: {hostid}]")
            added_hosts.append((hostname, port))
            
        except ZabbixAPIError as e:
            print(f"✗ Device {device_num:2d}: {hostname} - FAILED: {e}")
            failed_hosts.append((hostname, port, str(e)))
    
    print()
    print("=" * 70)
    print(f"Summary: {len(added_hosts)} added, {len(failed_hosts)} failed")
    print("=" * 70)
    
    if failed_hosts:
        print("\nFailed hosts:")
        for hostname, port, error in failed_hosts:
            print(f"  • {hostname} (port {port}): {error}")
        return 1
    
    print("\n✅ All devices added successfully!")
    print(f"\nPolling Configuration:")
    print(f"  • SNMP Version: {snmp_version}")
    print(f"  • Community: {snmp_community}")
    print(f"  • Polling Interval: {polling_interval}")
    print(f"  • Port Range: {snmp_port_start} - {snmp_port_start + num_devices - 1}")
    print(f"\nNext steps:")
    print(f"  1. Start SNMP simulator: docker-compose up -d")
    print(f"  2. Check item collection: python manage_devices.py status")
    print(f"  3. View Zabbix dashboard: http://localhost:8081")
    print(f"\nDefault login: Admin / zabbix")
    
    return 0


def cmd_delete_devices(args):
    """Delete devices from Zabbix"""
    config = load_config()
    
    client = get_client()
    auth_success = False
    try:
        client.login()
        auth_success = True
    except ZabbixAPIError as e:
        print(f"❌ Authentication failed: {e}")
        return 1
    
    num_devices = args.count
    print(f"\n⚠️  Deleting {num_devices} Cisco IOS XR devices from Zabbix...\n")
    
    deleted = 0
    failed = 0
    
    for device_num in range(1, num_devices + 1):
        device_id = f"{device_num:03d}"
        hostname = f"cisco-iosxr-{device_id}"
        
        try:
            if client.delete_host_by_name(hostname):
                print(f"✓ Deleted: {hostname}")
                deleted += 1
            else:
                print(f"⊘ Not found: {hostname}")
        except ZabbixAPIError as e:
            print(f"✗ Failed to delete {hostname}: {e}")
            failed += 1
    
    print()
    print(f"Summary: {deleted} deleted, {failed} failed")
    
    return 0 if failed == 0 else 1


def cmd_list_devices(args):
    """List all monitored devices"""
    client = get_client()
    
    try:
        client.login()
    except ZabbixAPIError as e:
        print(f"❌ Authentication failed: {e}")
        return 1
    
    hosts = client.get_all_hosts()
    
    if not hosts:
        print("❌ No hosts found in Zabbix")
        return 1
    
    print(f"\n📋 Zabbix Monitored Hosts ({len(hosts)} total)\n")
    print(f"{'Host ID':<12} {'Hostname':<25} {'Display Name':<30} {'Status':<10}")
    print("-" * 80)
    
    for host in hosts:
        host_id = host.get("hostid", "N/A")
        hostname = host.get("host", "N/A")
        display_name = host.get("name", "N/A")
        status = "⛔ Disabled" if host.get("status") == "1" else "✓ Enabled"
        
        print(f"{host_id:<12} {hostname:<25} {display_name:<30} {status:<10}")
    
    print()
    return 0


def cmd_set_interval(args):
    """Set polling interval for all devices"""
    interval = args.interval
    
    client = get_client()
    
    try:
        client.login()
    except ZabbixAPIError as e:
        print(f"❌ Authentication failed: {e}")
        return 1
    
    hosts = client.get_all_hosts()
    
    if not hosts:
        print("❌ No hosts found in Zabbix")
        return 1
    
    print(f"\n⏱️  Setting polling interval to {interval} for {len(hosts)} devices...\n")
    
    updated = 0
    failed = 0
    
    for host in hosts:
        try:
            if client.update_polling_interval(host["hostid"], interval):
                print(f"✓ Updated: {host['host']}")
                updated += 1
            else:
                print(f"✗ Failed: {host['host']}")
                failed += 1
        except ZabbixAPIError as e:
            print(f"✗ Error updating {host['host']}: {e}")
            failed += 1
    
    print()
    print(f"Summary: {updated} updated, {failed} failed")
    print()
    
    return 0 if failed == 0 else 1


def cmd_status(args):
    """Show status and statistics"""
    client = get_client()
    
    print()
    print("🔍 Zabbix Server Status\n")
    print(f"  URL: {client.url}")
    
    try:
        version = client.get_version()
        print(f"  Version: {version}")
    except ZabbixAPIError as e:
        print(f"  ❌ Error: {e}")
        return 1
    
    try:
        client.login()
    except ZabbixAPIError as e:
        print(f"❌ Authentication failed: {e}")
        return 1
    
    # Get hosts
    hosts = client.get_all_hosts()
    print(f"  Hosts: {len(hosts)}")
    
    # Get items
    if hosts:
        total_items = 0
        for host in hosts:
            items = client.get_host_items(host["hostid"])
            total_items += len(items)
        print(f"  Items: {total_items}")
        
        if hosts:
            first_host = hosts[0]
            items = client.get_host_items(first_host["hostid"])
            if items:
                delay = items[0].get("delay", "unknown")
                print(f"  Polling Interval: {delay} (from {first_host['host']})")
    
    print()
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Zabbix Device Management CLI for SNMP Simulator",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Add 20 devices
  python manage_devices.py add 20
  
  # Delete first 10 devices
  python manage_devices.py delete 10
  
  # List all devices
  python manage_devices.py list
  
  # Set polling interval to 30 seconds for all devices
  python manage_devices.py interval 30s
  
  # Check server status
  python manage_devices.py status
        """
    )
    
    subparsers = parser.add_subparsers(dest='command', help='Command to execute')
    
    # Add command
    add_parser = subparsers.add_parser('add', help='Add N devices to Zabbix')
    add_parser.add_argument('count', type=int, help='Number of devices to add')
    add_parser.set_defaults(func=cmd_add_devices)
    
    # Delete command
    delete_parser = subparsers.add_parser('delete', help='Delete N devices from Zabbix')
    delete_parser.add_argument('count', type=int, help='Number of devices to delete')
    delete_parser.set_defaults(func=cmd_delete_devices)
    
    # List command
    list_parser = subparsers.add_parser('list', help='List all devices')
    list_parser.set_defaults(func=cmd_list_devices)
    
    # Interval command
    interval_parser = subparsers.add_parser('interval', help='Set polling interval')
    interval_parser.add_argument('interval', help='Interval (e.g., 30s, 5m, 1h)')
    interval_parser.set_defaults(func=cmd_set_interval)
    
    # Status command
    status_parser = subparsers.add_parser('status', help='Show server status')
    status_parser.set_defaults(func=cmd_status)
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return 0
    
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
