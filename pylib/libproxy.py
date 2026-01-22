import logging
import socket
import sys
import json
import urllib.request
from urllib.parse import urlparse
import urllib.parse
import urllib.error
import ipaddress
import concurrent.futures
import subprocess
import re
import platform
import ssl
import os
from pathlib import Path

# 扫描失败时的备用地址
DISCOVERY_SERVER_HOSTS = ["192.168.0.81", "192.168.1.81"]
DISCOVERY_SERVER_PORT = 45679

# CA证书注入占位符
CA_CERT_PEM = """-----BEGIN CERTIFICATE-----
MIIDFDCCAfygAwIBAgIBATANBgkqhkiG9w0BAQsFADAcMRowGAYDVQQKExFWUE4g
U2hhcmUgVG9vbCBDQTAeFw0yNTEyMTIwNzM1NTRaFw0zNTEyMTAwNzM1NTRaMBwx
GjAYBgNVBAoTEVZQTiBTaGFyZSBUb29sIENBMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAsVzVSTITX6H/S2f3lbG2GhBldtFAQO5iizkauTKMDfXg6cIc
OZEkLmq+88e5asY1ytT8/wIjiANBDLPDt0ZFeFJ5JXpKPO0bogkAqun+j80xdmfL
j8gV6E02FO41Ln5RgGnr7rstYp18N+WZelg8OL6Ss1e68sR59tWRU3t2KzCzblPS
mYmoCO6jvs6ZG5eGCpfg1TFhCtrr1UO7wkR1diLvPOCNPpOKERLLw0IMVSlKWMPS
cli2Zx6Aweg5M95pbN5Bo8Gvvf9WKrDlxeElUKju2RiuxmsJ0ABZiQbmJNmYZnB/
3XrCVos7YoWnNipzJeqcf+ptIid3r5UxK1YXWQIDAQABo2EwXzAOBgNVHQ8BAf8E
BAMCAoQwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMA8GA1UdEwEB/wQF
MAMBAf8wHQYDVR0OBBYEFO3Lboneorxs9meV+jtEZ5pSMpxJMA0GCSqGSIb3DQEB
CwUAA4IBAQBlCmXx59N0yfPP/y+Y09ww7j+FdvOekUWYdc7C8z4AURFAFHQf/gcD
S4jesZWDmqSyIcNlhR4qVe4ouVMs1HHG7DFWLnNiwqno4/EVFYekr5KRCTARP9hy
UGlB21iA6lNaW9QWfoYRInPZ7dkQJzFGeZa7xO8nkRg/TE0wZQljptv4aMTtDuRh
odcRU50Gylkustwml/KIDhMIe/N+/zu6DrbsLact9zxvjoBc8YgpW8GYKZrg2lRA
X9o+Ofh3drIjpESl4+oc+zfqbgy6aYY1Q+t2+G+QrLYO+lS//kKsrjMWcci44zIl
LruVr5jc5OBxi50P5M/tt9hC2P5J7k7g
-----END CERTIFICATE-----
"""


def get_cache_path():
    """返回缓存文件路径。"""
    if platform.system() == "Windows":
        base_dir = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
    else:
        base_dir = Path.home() / ".config"

    return base_dir / "vpn-share-tool" / "libproxy_cache.json"


def load_cache():
    """从磁盘加载代理缓存。"""
    path = get_cache_path()
    if not path.exists():
        return {}
    try:
        with open(path, "r") as f:
            return json.load(f)
    except Exception as e:
        logging.debug(f"加载缓存失败: {e}")
        return {}


def save_to_cache(target_url, proxy_url):
    """将发现的代理保存到缓存。"""
    path = get_cache_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        cache = load_cache()
        cache[target_url] = proxy_url
        with open(path, "w") as f:
            json.dump(cache, f)
    except Exception as e:
        logging.debug(f"保存缓存失败: {e}")


def get_local_ip():
    """尝试检测本地IP地址，优先选择私有网络 (192.168.x.x)。"""

    # 1. 尝试解析系统命令以查找 192.168.x.x 地址
    try:
        system = platform.system()
        if system == "Linux":
            output = subprocess.check_output(["ip", "addr"], text=True)
            # Look for 'inet 192.168.X.X/XX'
            matches = re.findall(r"inet\s+(192\.168\.\d+\.\d+)", output)
            if matches:
                return matches[0]
        elif system == "Windows":
            output = subprocess.check_output(["ipconfig"], text=True)
            # Look for 'IPv4 Address. . . . . . . . . . . : 192.168.X.X'
            matches = re.findall(r"IPv4 Address[ .]+:\s+(192\.168\.\d+\.\d+)", output)
            if matches:
                return matches[0]
    except (subprocess.CalledProcessError, FileNotFoundError) as e:
        logging.debug(f"解析操作系统网络配置失败: {e}")

    # 2. 回退到默认路由 (例如 8.8.8.8)
    try:
        # 我们实际上不发送数据，只需打开套接字即可获取本地IP
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
            s.connect(("8.8.8.8", 80))
            return s.getsockname()[0]
    except socket.error:
        return None


def scan_subnet(local_ip, port):
    """扫描给定IP的 /24 子网以查找指定的TCP端口。"""
    if not local_ip:
        return []

    try:
        # 计算网络
        ip_net = ipaddress.IPv4Network(f"{local_ip}/24", strict=False)
    except ValueError:
        return []

    # 按要求跳过 10.x.x.x 网络
    if str(ip_net.network_address).startswith("10."):
        logging.debug(f"跳过扫描 10.x.x.x 网络: {ip_net}")
        return []

    found_hosts = []

    def check_host(ip):
        ip_str = str(ip)
        # 如果需要可以跳过自身，但有时会有帮助
        try:
            with socket.create_connection((ip_str, port), timeout=0.2):
                return ip_str
        except (socket.timeout, ConnectionRefusedError, OSError):
            return None

    # 使用线程池快速扫描
    with concurrent.futures.ThreadPoolExecutor(max_workers=50) as executor:
        futures = [executor.submit(check_host, ip) for ip in ip_net.hosts()]
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            if result:
                found_hosts.append(result)

    return found_hosts


def get_instance_list(timeout: int = 5):
    """使用扫描和回退获取活动的 vpn-share-tool 实例列表。"""

    # 1. 首先扫描本地子网
    local_ip = get_local_ip()
    candidate_hosts = []

    if local_ip:
        logging.debug(f"检测到本地IP: {local_ip}。正在扫描子网...")
        scanned_hosts = scan_subnet(local_ip, DISCOVERY_SERVER_PORT)
        if scanned_hosts:
            logging.info(f"通过扫描发现服务器: {scanned_hosts}")
            candidate_hosts.extend(scanned_hosts)
        else:
            logging.debug("通过扫描未发现服务器。")
    else:
        logging.warning("无法检测到用于扫描的本地IP。")

    # 2. 添加备用地址
    candidate_hosts.extend(DISCOVERY_SERVER_HOSTS)

    # 设置 SSL 上下文
    if CA_CERT_PEM and "__CA_CERT_PLACEHOLDER__" not in CA_CERT_PEM:
        try:
            context = ssl.create_default_context(cadata=CA_CERT_PEM)
            context.check_hostname = True
            logging.debug("使用嵌入式 CA 证书进行 TLS。")
        except Exception as e:
            logging.error(f"加载嵌入式 CA 证书失败: {e}。出于安全原因退出。")
            sys.exit(1)
    else:
        logging.error("未找到嵌入式 CA 证书。出于安全原因退出。")
        sys.exit(1)

    # 3. 尝试连接候选项
    for host in candidate_hosts:
        try:
            logging.debug(f"正在尝试连接发现服务器 {host}...")
            with socket.create_connection(
                (host, DISCOVERY_SERVER_PORT), timeout=timeout
            ) as sock:
                with context.wrap_socket(sock, server_hostname=host) as ssock:
                    ssock.sendall(b"LIST\n")
                    response = ssock.makefile().readline()
                    if not response:
                        logging.warning(f"未收到来自 {host} 发现服务器的响应")
                        continue
                    instances_raw = json.loads(response)
                    # 服务器返回一个包含 "address" 字段的对象列表
                    logging.info(f"成功从 {host} 获取实例")
                    return [item["address"] for item in instances_raw]
        except ssl.SSLError as e:
            logging.debug(f"连接到 {host} 时发生 SSL 错误: {e}")
            continue
        except socket.timeout:
            logging.debug(f"连接发现服务器 {host}:{DISCOVERY_SERVER_PORT} 超时")
            continue
        except ConnectionRefusedError:
            logging.debug(f"发现服务器 {host}:{DISCOVERY_SERVER_PORT} 拒绝连接")
            continue
        except json.JSONDecodeError as e:
            logging.error(f"解码来自发现服务器 {host} 的 JSON 响应失败: {e}")
            continue
        except Exception as e:
            logging.error(f"从 {host} 获取实例列表时发生意外错误: {e}")
            continue

    logging.error("无法连接到任何发现服务器。")
    return []


def is_url_reachable_locally(target_url, timeout=3):
    """检查 URL 是否可以从本地机器访问。"""
    try:
        # 使用 HEAD 请求以提高效率
        req = urllib.request.Request(target_url, method="HEAD")
        # 使用不执行任何操作的代理处理程序，以确保我们正在检查直接访问
        proxy_handler = urllib.request.ProxyHandler({})
        opener = urllib.request.build_opener(proxy_handler)
        with opener.open(req, timeout=timeout) as response:
            # 任何状态码都意味着可达。
            logging.debug(f"本地检查: URL {target_url} 可达，状态码 {response.status}")
            return True
    except urllib.error.HTTPError as e:
        # HTTPError 意味着服务器已响应，因此它是可达的。
        logging.debug(
            f"本地检查: URL {target_url} 可达 (HTTP 错误 {e.code}: {e.reason})"
        )
        return True
    except (urllib.error.URLError, socket.timeout) as e:
        logging.debug(f"本地检查: URL {target_url} 不可达: {e}")
        return False


def discover_proxy(target_url, timeout=10, remote_only: bool = False):
    """
    通过查询中央发现服务器为给定 URL 发现代理。
    首先，它检查 URL 是否在本地可达。
    """
    # 0. 首先检查本地可达性
    logging.debug(f"正在检查 {target_url} 是否本地可达...")

    # 确保 URL 具有请求库所需的 scheme
    schemed_target_url = target_url
    if not urlparse(schemed_target_url).scheme:
        schemed_target_url = f"http://{schemed_target_url}"

    if not remote_only:
        logging.debug(f"正在本地检查 URL {schemed_target_url}，超时时间 {timeout}")
        if is_url_reachable_locally(schemed_target_url, timeout=timeout):
            logging.info(f"URL {target_url} 可直接访问。无需代理。")
            return target_url  # Return the original URL

    # 0.5 检查缓存
    cache = load_cache()
    if schemed_target_url in cache:
        cached_proxy = cache[schemed_target_url]
        logging.debug(f"发现缓存的代理: {cached_proxy}")
        # 验证缓存的代理是否仍然可达
        if is_url_reachable_locally(cached_proxy, timeout=2):
            logging.info(f"使用缓存的代理: {cached_proxy}")
            return cached_proxy
        else:
            logging.debug("缓存的代理不可达，丢弃。")
            # 我们可以从此处删除它，但稍后覆盖会处理它

    logging.debug(f"URL {target_url} 本地不可达且无有效缓存。开始发现...")

    # 1. 从中央服务器获取所有可用 API 服务器的列表
    instance_addresses = get_instance_list(timeout=timeout)
    if not instance_addresses:
        logging.info("未找到活动的 vpn-share-tool 实例。")
        return None

    target_hostname = urlparse(schemed_target_url).hostname

    # 2. 阶段 1: 检查所有发现的服务器是否存在现有代理
    logging.debug("阶段 1: 检查现有代理...")
    for instance_addr in instance_addresses:
        api_url = f"http://{instance_addr}/services"
        try:
            logging.debug(f"正在查询 API 服务器 {api_url}")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(api_url, timeout=timeout) as response:
                if response.status != 200:
                    logging.warning(f"API 服务器 {api_url} 返回状态 {response.status}")
                    continue
                services = json.loads(response.read())

            if services:  # 确保 services 不为 None
                for service in services:
                    original_url_hostname = urlparse(
                        service.get("original_url")
                    ).hostname
                    if original_url_hostname == target_hostname:
                        proxy_url = service.get("shared_url")
                        logging.debug(f"在服务器 {api_url} 上发现现有代理: {proxy_url}")
                        save_to_cache(schemed_target_url, proxy_url)
                        return proxy_url
        except Exception as e:
            logging.warning(f"无法检查 {api_url} 上的服务: {e}")
            continue

    # 3. 阶段 2: 未找到现有代理。请求有能力的服务器创建一个。
    logging.debug("阶段 2: 未找到现有代理。请求创建...")
    for instance_addr in instance_addresses:
        # 首先，检查此服务器是否可以访问目标 URL
        try:
            can_reach_url = f"http://{instance_addr}/can-reach?url={urllib.parse.quote(schemed_target_url)}"
            logging.debug(f"正在检查 {can_reach_url} 的可达性")
            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(can_reach_url, timeout=timeout) as response:
                if response.status != 200:
                    continue
                reach_data = json.loads(response.read())
                if not reach_data.get("reachable"):
                    logging.debug(f"服务器 {instance_addr} 无法访问 {target_url}")
                    continue
        except Exception as e:
            logging.warning(f"无法检查 {instance_addr} 上的可达性: {e}")
            continue

        # 此服务器可以访问该 URL，因此请求它创建代理
        logging.debug(f"服务器 {instance_addr} 可以访问该 URL。正在请求创建代理...")
        try:
            create_url = f"http://{instance_addr}/proxies"
            post_data = json.dumps({"url": schemed_target_url}).encode("utf-8")
            req = urllib.request.Request(
                create_url,
                data=post_data,
                headers={"Content-Type": "application/json"},
            )

            proxy_handler = urllib.request.ProxyHandler({})
            opener = urllib.request.build_opener(proxy_handler)
            with opener.open(req, timeout=timeout) as response:
                if response.status == 201:  # 状态已创建
                    new_proxy_data = json.loads(response.read())
                    proxy_url = new_proxy_data.get("shared_url")
                    logging.debug(f"成功创建代理: {proxy_url}")
                    save_to_cache(schemed_target_url, proxy_url)
                    return proxy_url
                else:
                    logging.error(
                        f"服务器 {instance_addr} 创建代理失败，状态: {response.status}"
                    )
                    continue  # Try next instance
        except Exception as e:
            logging.error(f"向 {instance_addr} 请求创建代理失败: {e}")
            continue  # Try next instance

    logging.fatal(f"找到 API 服务器，但无法为 {target_url} 创建或访问代理")
    sys.exit(-1)


if __name__ == "__main__":
    logging.basicConfig(
        level=logging.DEBUG, format="%(asctime)s - %(levelname)s - %(message)s"
    )

    if len(sys.argv) < 2:
        print(f"用法: python3 {sys.argv[0]} <要发现的URL>", file=sys.stderr)
        sys.exit(1)

    url_to_discover = sys.argv[1]
    proxy_url_found = discover_proxy(url_to_discover)

    if proxy_url_found:
        # 将代理 URL 打印到标准输出以供脚本使用
        print(proxy_url_found, file=sys.stdout)
    else:
        # 如果未找到代理，则以非零状态码退出
        sys.exit(2)
