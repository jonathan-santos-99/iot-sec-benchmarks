#!/usr/bin/env python3

import subprocess
import sys

devices = [
    "server",
    "backend",
    "sensor-001"
]

def generate_keys(out_folder, device):
    print(f"Generating keys for dispositivo {device}...")
    out_file = f"{out_folder}/{device}-key.pem"
    result = subprocess.run([
        "openssl",
        "genrsa" ,
        "-out"   , out_file,
        "2048"
    ])
    result.check_returncode()
    return out_file

def generate_csr(out_folder, device, key_file):
    print(f"Generating CSR for device {device}...")

    out_file = f"{out_folder}/{device}-csr.pem"
    result = subprocess.run([
        "openssl",
        "req"    , "-new",
        "-out"   , out_file,
        "-key"   , key_file,
        "-subj"  , "/CN=localhost"
    ])

    result.check_returncode()
    return out_file

def sign(out_folder, device, csr_file, ca_crt, ca_key):
    print(f"Genrating crt for device {device}...")

    out = f"{out_folder}/{device}-crt.pem"
    result = subprocess.run([
        "openssl"         ,
        "x509"            ,
        "-req"            ,
        "-in"             , csr_file,
        "-CA"             , ca_crt  ,
        "-CAkey"          , ca_key  ,
        "-CAcreateserial" ,
        "-out"            , out   ,
        "-days"           , "3650",
    ])

    result.check_returncode()

def main():
    out_folder, ca_crt, ca_key = sys.argv[1:]

    for device in devices:
        key_file_path = generate_keys(out_folder, device)
        csr_file = generate_csr(out_folder, device, key_file_path)
        sign(out_folder, device, csr_file, ca_crt, ca_key)

    print("All finished ;)")


if __name__ == '__main__':
    main()