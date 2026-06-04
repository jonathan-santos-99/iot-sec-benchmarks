#!/usr/bin/env python3

import getpass
import hashlib
import sys


def usage(program_name: str):
    print(f"Usage: {program_name} pwfile")
    sys.exit(1)

def sha256(s: str) -> str:
    return hashlib.sha256(s.encode("utf-8")).hexdigest()

def main():
    args = sys.argv
    program_name = args[0]
    if len(args) != 2:
        usage(program_name)

    pwfile   = args[1]
    username = input("Username: ")
    password = getpass.getpass("Password: ")
    digest   = sha256(password)

    with open(pwfile, "a") as f:
        f.write(f"{username};{digest}\n")

    print(f"User `{username}` add successfully to file `{pwfile}`!")

if __name__ == '__main__':
    main()