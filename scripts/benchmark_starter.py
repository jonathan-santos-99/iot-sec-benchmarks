import subprocess
import sys

benchmarks = {
    # just encrypted
    "plain text"                 : (1, 0, 0, False),
    "aes"                        : (2, 1, 0, False),
    "chacha20"                   : (3, 2, 0, False),

    # checksum
    "plain text (checksum)"      : (4, 0, 1, False),
    "aes (checksum)"             : (5, 1, 1, False),
    "chacha20 (checksum)"        : (6, 2, 1, False),

    # tls
    "plain text (TLS)"           : (7, 0, 0, True),
    "aes (TLS)"                  : (8, 1, 0, True),
    "chacha20 (TLS)"             : (9, 2, 0, True),

    # tls + checksum
    "plain text (checksum+TLS)"  : (10, 0, 1, True),
    "aes (checksum+TLS)"         : (11, 1, 1, True),
    "chacha20 (checksum+TLS)"    : (12, 2, 1, True),
}

def main():
    topic    = sys.argv[1]
    duration = sys.argv[2]
    user     = sys.argv[3]
    password = sys.argv[4]
    ca_file  = sys.argv[5]

    for k in benchmarks:
        print(f"Bechmark for {k} queued!")
        id, algorithm, checksum, tls = benchmarks[k]
        if not tls:
            subprocess.run([
                "mosquitto_pub",
                "-t", topic,
                "-m", f"{id};{algorithm};{duration};{checksum}",
                "-u", user,
                "-P", password
            ])
        else:
            subprocess.run([
                "mosquitto_pub",
                "-p", "8883",
                "--cafile", ca_file,
                "-t", f"tls/{topic}",
                "-m", f"{id};{algorithm};{duration};{checksum}",
                "-u", user,
                "-P", password
            ])

if __name__ == '__main__':
    main()