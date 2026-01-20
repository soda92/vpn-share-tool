use std::collections::HashMap;
use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use local_ip_address::local_ip;
use native_tls::{Certificate, TlsConnector};
use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};
use threadpool::ThreadPool;
use url::Url;

const DISCOVERY_SERVER_PORT: u16 = 45679;
const DISCOVERY_HOSTS: &[&str] = &["192.168.0.81", "192.168.1.81"];
const CA_CERT_PLACEHOLDER: &str = r#"-----BEGIN CERTIFICATE-----
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
"#;

#[derive(Serialize, Deserialize, Debug)]
struct Instance {
    address: String,
}

#[derive(Serialize, Deserialize, Debug)]
struct Service {
    original_url: String,
    shared_url: String,
}

#[derive(Serialize, Deserialize, Debug)]
struct ReachResponse {
    reachable: bool,
}

#[derive(Serialize, Deserialize, Debug)]
struct CreateProxyRequest {
    url: String,
}

fn get_cache_path() -> Option<PathBuf> {
    let mut path = if cfg!(target_os = "windows") {
        dirs::config_dir().map(|p| p.join("vpn-share-tool"))
    } else {
        dirs::home_dir().map(|p| p.join(".config").join("vpn-share-tool"))
    }?;
    path.push("libproxy_cache.json");
    Some(path)
}

fn load_cache() -> HashMap<String, String> {
    if let Some(path) = get_cache_path() {
        if path.exists() {
            if let Ok(file) = fs::File::open(path) {
                if let Ok(cache) = serde_json::from_reader(file) {
                    return cache;
                }
            }
        }
    }
    HashMap::new()
}

fn save_to_cache(target: &str, proxy: &str) {
    if let Some(path) = get_cache_path() {
        if let Some(parent) = path.parent() {
            let _ = fs::create_dir_all(parent);
        }
        let mut cache = load_cache();
        cache.insert(target.to_string(), proxy.to_string());
        if let Ok(file) = fs::File::create(path) {
            if let Err(e) = serde_json::to_writer_pretty(file, &cache) {
                // Log the error using standard log crate if initialized, or just eprintln
                eprintln!("Failed to write cache: {}", e);
            }
        }
    }
}

fn scan_subnet(local_ip_str: &str, port: u16) -> Vec<String> {
    let parts: Vec<&str> = local_ip_str.split('.').collect();
    if parts.len() != 4 {
        return vec![];
    }

    // Skip 10.x.x.x
    if parts[0] == "10" {
        return vec![];
    }

    let prefix = format!("{}.{}.{}.", parts[0], parts[1], parts[2]);
    let found_hosts = Arc::new(Mutex::new(Vec::new()));
    let pool = ThreadPool::new(50);

    for i in 1..255 {
        let ip = format!("{}{}", prefix, i);
        let hosts = found_hosts.clone();
        pool.execute(move || {
            if let Ok(mut addrs) = format!("{}:{}", ip, port).to_socket_addrs() {
                if let Some(addr) = addrs.next() {
                    if TcpStream::connect_timeout(&addr, Duration::from_millis(200)).is_ok() {
                        hosts.lock().unwrap().push(ip);
                    }
                }
            }
        });
    }
    pool.join();

    let res = found_hosts.lock().unwrap().clone();
    res
}

fn get_tls_connector() -> Option<TlsConnector> {
    let mut builder = TlsConnector::builder();
    builder.danger_accept_invalid_hostnames(true); // Ignore hostname mismatch (IP vs Cert Hostname)

    let ca_pem = if CA_CERT_PLACEHOLDER.contains("__CA_CERT_PLACEHOLDER__") {
        // Try env var
        std::env::var("VPN_SHARE_TOOL_CA_PATH")
            .ok()
            .and_then(|p| fs::read_to_string(p).ok())
    } else {
        Some(CA_CERT_PLACEHOLDER.to_string())
    };

    if let Some(pem) = ca_pem {
        if let Ok(cert) = Certificate::from_pem(pem.as_bytes()) {
            builder.add_root_certificate(cert);
        }
    }

    builder.build().ok()
}

fn get_instance_list(timeout: Duration) -> Vec<String> {
    let mut candidates = Vec::new();
    if let Ok(ip) = local_ip() {
        let ip_str = ip.to_string();
        candidates.extend(scan_subnet(&ip_str, DISCOVERY_SERVER_PORT));
    }
    for h in DISCOVERY_HOSTS {
        candidates.push(h.to_string());
    }

    let connector = get_tls_connector().unwrap_or_else(|| TlsConnector::new().unwrap());

    for host in candidates {
        let addr = format!("{}:{}", host, DISCOVERY_SERVER_PORT);
        if let Ok(stream) =
            TcpStream::connect_timeout(&addr.to_socket_addrs().unwrap().next().unwrap(), timeout)
        {
            // Need to set read timeout
            let _ = stream.set_read_timeout(Some(timeout));

            if let Ok(mut tls_stream) = connector.connect(host.as_str(), stream) {
                if tls_stream.write_all(b"LIST\n").is_ok() {
                    let mut reader = BufReader::new(tls_stream);
                    let mut line = String::new();
                    if reader.read_line(&mut line).is_ok() {
                        if let Ok(instances) = serde_json::from_str::<Vec<Instance>>(&line) {
                            return instances.into_iter().map(|i| i.address).collect();
                        }
                    }
                }
            }
        }
    }
    Vec::new()
}

fn is_reachable(url: &str, timeout: Duration) -> bool {
    let client = Client::builder()
        .timeout(timeout)
        .no_proxy()
        .build()
        .unwrap();
    client.head(url).send().is_ok()
}

pub fn discover_proxy(target_url: &str, timeout_secs: u64, remote_only: bool) -> Option<String> {
    let timeout = Duration::from_secs(timeout_secs);
    let mut url_parsed = Url::parse(target_url).ok()?;
    if url_parsed.scheme() == "" {
        url_parsed.set_scheme("http").ok()?;
    }
    let target_url_str = url_parsed.to_string();

    if !remote_only {
        if is_reachable(&target_url_str, timeout) {
            return Some(target_url_str);
        }
    }

    let cache = load_cache();
    if let Some(proxy) = cache.get(&target_url_str) {
        if is_reachable(proxy, Duration::from_secs(2)) {
            return Some(proxy.clone());
        }
    }

    let instances = get_instance_list(timeout);
    if instances.is_empty() {
        return None;
    }

    let target_host = url_parsed.host_str()?;
    let client = Client::builder().timeout(timeout).build().ok()?;

    // Phase 1
    for addr in &instances {
        let api_url = format!("http://{}/services", addr);
        if let Ok(resp) = client.get(&api_url).send() {
            if let Ok(services) = resp.json::<Vec<Service>>() {
                for s in services {
                    if let Ok(u) = Url::parse(&s.original_url) {
                        if u.host_str() == Some(target_host) {
                            save_to_cache(&target_url_str, &s.shared_url);
                            return Some(s.shared_url);
                        }
                    }
                }
            }
        }
    }

    // Phase 2
    for addr in &instances {
        let can_reach = format!("http://{}/can-reach", addr);
        if let Ok(resp) = client
            .get(&can_reach)
            .query(&[("url", &target_url_str)])
            .send()
        {
            if let Ok(r) = resp.json::<ReachResponse>() {
                if !r.reachable {
                    continue;
                }

                let create_url = format!("http://{}/proxies", addr);
                let body = CreateProxyRequest {
                    url: target_url_str.clone(),
                };
                if let Ok(resp) = client.post(&create_url).json(&body).send() {
                    if resp.status().is_success() {
                        if let Ok(new_proxy) = resp.json::<Service>() {
                            save_to_cache(&target_url_str, &new_proxy.shared_url);
                            return Some(new_proxy.shared_url);
                        }
                    }
                }
            }
        }
    }

    None
}
