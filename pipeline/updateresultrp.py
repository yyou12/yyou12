#!/usr/bin/env python3
import argparse
import requests
from requests.adapters import HTTPAdapter
from urllib3.util import Retry
import urllib3
from urllib3.exceptions import InsecureRequestWarning
import yaml
import urllib.parse

class UpdateResultonRP:
    def __init__(self, args):
        if args.token == "":
            with open("secrets/rp/openshift-qe-reportportal.json") as f:
                token_f = yaml.safe_load(f)
                args.token = token_f["ginkgo_rp_mmtoken"]
        urllib3.disable_warnings(category=InsecureRequestWarning)
        self.session = requests.Session()
        self.session.headers["Authorization"] = "bearer {0}".format(args.token)
        self.session.verify = False
        retry = Retry(connect=3, backoff_factor=0.5)
        adapter = HTTPAdapter(max_retries=retry)
        self.session.mount('https://', adapter)
        self.session.mount('http://', adapter)
        self.session.trust_env = False

        self.launch_url = args.endpoint + "/v1/" + args.project + "/launch"
        self.item_url = args.endpoint + "/v1/" + args.project + "/item"
        self.args = args


    def isGolangLaunch(self, attrs):
        isGolang = False
        for attr in attrs:
            if attr["key"] == "launchtype" and attr["value"] == "golang":
            # in the future, maybe add launchtype=cucu and then add check attr["value"] != "cucu"
                isGolang = True
        return isGolang

    def getLaunchIdWithLaunchName(self, launchname, attrfilter=None):
        filter_url = self.launch_url + "?page.page=1&page.size=150&filter.eq.name=" + launchname
        try:
            r = self.session.get(url=filter_url)
            if (r.status_code != 200):
                raise Exception("get ID error: {0}".format(r.text))
            ids = []
            for ret in r.json()["content"]:
                if not self.isGolangLaunch(ret["attributes"]):
                    continue

                if attrfilter == None:
                    ids.append(ret["id"])
                else:
                    for attr in ret["attributes"]:
                        if attr["key"] == attrfilter["key"] and attr["value"] == attrfilter["value"]:
                            ids.append(ret["id"])
                            ids.append(ret["uuid"])
                            ids.append(ret["startTime"])
                            ids.append(ret["endTime"])
            if len(ids) == 0:
                raise Exception('no matched launch id')
            return ids
        except BaseException as e:
            print(e)
            return None


    def getItemId(self, launchid, itemtype, itemstatus):
        query_item_url = self.item_url + "?filter.eq.launchId={0}&filter.eq.type={1}&filter.eq.status={2}&isLatest=false&launchesLimit=0&page.size=200".format(launchid, itemtype, itemstatus)
        ids = []
        r = self.session.get(url=query_item_url)
        if (r.status_code != 200):
            print("can not get item with error {0} and code {1}".format(r.text, r.status_code))
            return ids

        if len(r.json()["content"]) == 0:
            print("no item match with launch name={0}, subteam={1} and status={2}".format(self.args.launchname, self.args.subteam, itemstatus))
            return ids

        for ret in r.json()["content"]:
            ids.append(ret["id"])
        
        return ids

    def ChangeToSuccessPerLuanch(self):
        existinglaunch = self.getLaunchIdWithLaunchName(self.args.launchname, {"key": "team", "value":self.args.subteam})
        if existinglaunch == None:
            print("no launch match with launch name={0} and subteam={1}".format(self.args.launchname, self.args.subteam))
            return
        item_ids = self.getItemId(existinglaunch[0], "STEP", "FAILED")
        for item_id in item_ids:
            print(item_id)
            update_tiem_url = self.item_url + "/{0}/update".format(item_id)
            itemdata = {
                "attributes": [
                    {
                    "key": "manually",
                    "value": "passed"
                    }
                ],
                "status": "PASSED"
            }
            r = self.session.put(url=update_tiem_url, json=itemdata)
            if (r.status_code != 200) and (r.status_code != 201):
                print("can not change status for {0}, please check launch={1} with subteam={2}".format(item_id,self.args.launchname, self.args.subteam))


if __name__ == "__main__":
    parser = argparse.ArgumentParser("updateresultrp.py")
    parser.add_argument("-a","--action", default="cls", choices={"cls"}, required=True)
    parser.add_argument("-e","--endpoint", default="https://reportportal-openshift.apps.ocp4.prod.psi.redhat.com/api")
    parser.add_argument("-t","--token", default="")
    parser.add_argument("-p","--project", default="ocp")

    parser.add_argument("-s","--subteam", default="")
    parser.add_argument("-l","--launchname", default="")
    args=parser.parse_args()

    updr = UpdateResultonRP(args)
    if args.action == "cls":
        updr.ChangeToSuccessPerLuanch()
    exit(0)

