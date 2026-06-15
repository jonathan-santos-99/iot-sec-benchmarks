#!/usr/bin/env python3

import json
import sys


def main():
    config_file = sys.argv[1]
    config = {}
    with open(config_file, "r") as f:
        config = json.load(f)

    out_file = "./esp32/mosquitto/main/topics.h"
    with open(out_file, "w") as f:
        print("#ifndef _TOPIC_H\n#define _TOPIC_H\n", file=f)
        print("typedef enum {", file=f)
        algs = [n for n in config]
        for i, alg in enumerate(algs):
            print(f"    {alg} = {i},", file=f)
        print("} Algorithm;", file=f)
        print(
"""
typedef struct {
    const char *name;
    const char *key;
} Topic_Info;
""", file=f)

        print("Topic_Info outbond_topics[] = {", file=f)
        for n in config:
            print("    [%s] = {" % n, file=f)
            print(f'        .name = "{config[n]['topic']}",', file=f)
            print(f'        .key  = "{config[n]['key']}",', file=f)
            print("    },", file=f)
        print("};\n", file=f)
        print("#endif // _TOPIC_H", file=f)



if __name__ == '__main__':
    main()