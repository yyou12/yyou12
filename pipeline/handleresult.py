#!/usr/bin/env python
import xml.dom.minidom
import argparse
import re

class TestResult:

    def removeMonitor(self, input, output):
        noderoot = xml.dom.minidom.parse(input)

        testsuites = noderoot.getElementsByTagName("testsuite")
        
        totalcasenum = testsuites[0].getAttribute("tests")
        failcasenum = testsuites[0].getAttribute("failures")
        
        cases = noderoot.getElementsByTagName("testcase")
        toBeRemove = None
        for case in cases:
            value = case.getAttribute("name")
            if "Monitor cluster while tests execute" in value:
                testsuites[0].setAttribute("tests", str(int(totalcasenum)-1))
                testsuites[0].setAttribute("failures", str(int(failcasenum)-1))
                toBeRemove = case
                break
        if toBeRemove is not None:
            noderoot.firstChild.removeChild(toBeRemove)

        with open(output, 'w+') as writer:
            noderoot.writexml(writer)

    def pirntResult(self, input):
        testsummary = {}
        result = ""
        noderoot = xml.dom.minidom.parse(input)
        cases = noderoot.getElementsByTagName("testcase")
        for case in cases:
            name = case.getAttribute("name")
            if "Monitor cluster while tests execute" in name:
                continue
            failure = case.getElementsByTagName("failure")
            if failure:
                result="FAIL"
            else:
                result="PASS"
            caseids = re.findall(r'\d{5,}-', name)
            if len(caseids) == 0:
                testsummary["the case title does not include case ID, so take case title:"+name.replace("'","").split("[Suite:")[0]] = {"result":result, "title":""}
            else:
                casetitle = name.split(caseids[-1])[1].split("[Suite:")[0]
                for i in caseids:
                    id = "OCP-"+i[:-1]
                    if id in testsummary and "FAIL" in testsummary[id]["result"]: #the case already execute with failure
                        result = testsummary[id]["result"]
                    testsummary[id] = {"result":result, "title":casetitle.replace("'","")}
        print("The Case Execution Summary:\\n ")
        [print(testsummary[k]["result"]+"  ", k.replace("'","")+"  ", testsummary[k]["title"]+"\\n") for k in sorted(testsummary.keys())] 

if __name__ == "__main__":
    parser = argparse.ArgumentParser("handleresult.py")
    parser.add_argument("-a","--action", default="get", choices={"replace", "get"}, required=True)
    parser.add_argument("-in","--input", default="", required=True)
    parser.add_argument("-o","--output", default="")
    args=parser.parse_args()

    
    testresult = TestResult()
    if args.input == "":
        print("please provide input paramter")
        exit(1)
    if args.action == "get":
        testresult.pirntResult(args.input)
        exit(0)

    if args.output == "":
        print("please provide output paramter")
        exit(1)
    if args.action == "replace":
        testresult.removeMonitor(args.input, args.output)
