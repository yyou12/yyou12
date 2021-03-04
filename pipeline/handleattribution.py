#!/usr/bin/env python
import yaml
import argparse

class AttributionParse:

    def __init__(self, content):
        self.content = content

    def get(self, para):
        value = ""
        parapath = para.split(":")
        value = self.content
        try:
            for i in parapath:
                value = value[i]
        except BaseException:
            value = "failtogetvalue"
        return value

if __name__ == "__main__":
    parser = argparse.ArgumentParser("handleattribution.py")
    parser.add_argument("-a","--action", default="get", choices={"get"}, required=True)
    parser.add_argument("-y","--yaml", default="", required=True)
    parser.add_argument("-p","--para", default="", required=True)
    args=parser.parse_args()

    try:
        yamlcontent = yaml.safe_load(args.yaml)
    except yaml.constructor.ConstructorError as e:
        print("failtogetvalue")
        exit(1)

    config = AttributionParse(yamlcontent)
    print(config.get(args.para))
