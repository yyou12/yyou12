#!/usr/bin/env python3
import argparse
import requests
from requests.adapters import HTTPAdapter
from urllib3.util import Retry
import urllib3
from urllib3.exceptions import InsecureRequestWarning
from datetime import datetime, timedelta
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

    def makeLaunchFilterUrl(self, filters=None):
        filter_url = self.launch_url + "?page.page=1&page.size=500"
        if filters["launchName"] != "":
            filter_url = filter_url + "&filter.cnt.name=" + filters["launchName"].replace(" ", "")

        if filters["attributeKey"] != "":
            filter_url = filter_url + "&filter.has.attributeKey=" + urllib.parse.quote(filters["attributeKey"].replace(" ", ""))

        if filters["subTeam"] != "" and filters["attributeValue"] != "":
            att_value = filters["subTeam"] + "," + filters["attributeValue"]
            filter_url = filter_url + "&filter.has.attributeValue=" + urllib.parse.quote(att_value)
        elif filters["subTeam"] != "":
            filter_url = filter_url + "&filter.has.attributeValue=" + urllib.parse.quote(filters["subTeam"])
        elif filters["attributeValue"] != "":
            filter_url = filter_url + "&filter.has.attributeValue=" + urllib.parse.quote(filters["attributeValue"])

        if filters["timeDuration"] != "":
            if len(filters["timeDuration"].split(",")) > 1:
                start_time = filters["timeDuration"].split(",")[0]
                end_time = filters["timeDuration"].split(",")[1]
            else:
                start_time = filters["timeDuration"].split(",")[0]
                end_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
            timediff = datetime.strptime(start_time, "%Y-%m-%d %H:%M:%S").strftime('%s.%f')
            sttimestamp = int(float(timediff)*1000)
            timediff = datetime.strptime(end_time, "%Y-%m-%d %H:%M:%S").strftime('%s.%f')
            edtimestamp = int(float(timediff)*1000+1000)
            filter_url = filter_url + "&filter.btw.startTime=" + urllib.parse.quote(str(sttimestamp)+","+str(edtimestamp))

        filter_url = filter_url + "&filter.gte.statistics" + urllib.parse.quote("$executions$failed") + "=" +filters["failedNum"]

        print(filter_url)
        return filter_url


    def getLaunchId(self, filters=None):
        filter_url = self.makeLaunchFilterUrl(filters)

        try:
            r = self.session.get(url=filter_url)
            if (r.status_code != 200):
                raise Exception("get ID error: {0} with code {1}".format(r.text, r.status_code))
            ids = []
            for ret in r.json()["content"]:
                if not self.isGolangLaunch(ret["attributes"]):
                    continue
                ids.append(ret["id"])

            if len(ids) == 0:
                raise Exception('no matched launch id')
            return ids
        except BaseException as e:
            print(e)
            return None

    def getItemId(self, launchid, itemtype, itemstatus):
        query_item_url = self.item_url + "?filter.eq.launchId={0}&filter.eq.type={1}&filter.eq.status={2}&isLatest=false&launchesLimit=0&page.size=500".format(launchid, itemtype, itemstatus)
        ids = []
        r = self.session.get(url=query_item_url)
        if (r.status_code != 200):
            print("can not get item with error {0} and code {1}".format(r.text, r.status_code))
            return ids

        if len(r.json()["content"]) == 0:
            print("no item match with launch ID={0} and status={1}".format(launchid, itemstatus))
            return ids

        for ret in r.json()["content"]:
            ids.append(ret["id"])

        return ids

    def updateItemStatus(self, itemid, itemstatus):
        update_tiem_url = self.item_url + "/{0}/update".format(itemid)
        itemdata = {
            "attributes": [
                {
                "key": "manually",
                "value": itemstatus
                }
            ],
            "status": itemstatus
        }
        r = self.session.put(url=update_tiem_url, json=itemdata)
        return r.status_code

    def updateItemPerLaunch(self, launchid, itemstatus):
        item_ids = self.getItemId(launchid, "STEP", itemstatus)
        for item_id in item_ids:
            # print(item_id)
            ret_code = self.updateItemStatus(item_id, "passed")
            if (ret_code != 200) and (ret_code != 201):
                print("can not change status for item ID={0} in launch with ID={1}. please rerun or manually change status".format(item_id, launchid))

    def ChangeToSuccess(self):
        filters = {
            "launchName": self.args.launchname,
            "timeDuration": self.args.timeduration,
            "subTeam": self.args.subteam,
            "attributeKey": self.args.attrkey,
            "attributeValue": self.args.attrvalue,
            "failedNum": "1"
        }
        existinglaunchs = self.getLaunchId(filters)
        print(existinglaunchs)
        if existinglaunchs == None:
            print("no launch match")
            return
        for launch in existinglaunchs:
            self.updateItemPerLaunch(launch, "FAILED")


if __name__ == "__main__":
    parser = argparse.ArgumentParser("updateresultrp.py")
    parser.add_argument("-a","--action", default="cls", choices={"cls"}, required=True)
    parser.add_argument("-e","--endpoint", default="https://reportportal-openshift.apps.ocp4.prod.psi.redhat.com/api")
    parser.add_argument("-t","--token", default="")
    parser.add_argument("-p","--project", default="ocp")

    parser.add_argument("-l","--launchname", default="")
    parser.add_argument("-s","--subteam", default="")
    parser.add_argument("-td","--timeduration", default="")
    parser.add_argument("-ak","--attrkey", default="")
    parser.add_argument("-av","--attrvalue", default="")
    parser.add_argument("-fn","--failednum", default="0")
    args=parser.parse_args()

    updr = UpdateResultonRP(args)
    if args.action == "cls":
        updr.ChangeToSuccess()
    exit(0)

