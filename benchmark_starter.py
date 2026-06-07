import subprocess
import sys
import time

benchmarks = {
    "plain text"            : (1, 0, 0),
    "aes"                   : (2, 1, 0),
    "chacha20"              : (3, 2, 0),
    "plain text (checksum)" : (4, 0, 1),
    "aes (checksum)"        : (5, 1, 1),
    "chacha20 (checksum)"   : (6, 2, 1),
}

def main():
    topic    = sys.argv[1]
    duration = sys.argv[2]
    user     = sys.argv[3]
    password = sys.argv[4]

    for k in benchmarks:
        id, algorithm, checksum = benchmarks[k]
        print(f"Running bechmark for {k}")
        subprocess.run([
            "mosquitto_pub",
            "-t", topic,
            "-m", f"{id};{algorithm};{duration};{checksum}",
            "-u", user,
            "-P", password
        ])
        time.sleep(float(duration))

if __name__ == '__main__':
    main()