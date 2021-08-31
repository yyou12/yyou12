#!/usr/bin/python3
# author : jitli cover anli
import os
import re
import time
import sys
import yaml
import urllib3
import requests
import argparse
import xml.etree.ElementTree as ET
import gspread
from xml.sax.saxutils import unescape
from bs4 import BeautifulSoup

from oauth2client.service_account import ServiceAccountCredentials

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

class Logger(object):
    def __init__(self, filename="Default.log"):
        self.terminal = sys.stdout
        self.log = open(filename, "a")

    def write(self, message):
        self.terminal.write(message)
        self.log.write(message)

    def flush(self):
        pass
path = os.path.abspath(os.path.dirname(__file__))
type = sys.getfilesystemencoding()
sys.stdout = Logger('log')


def ginkgo_google_report():
    #####  get url && jenkins xml file

    ## get jenkins ginkgovmtest url
    url = 'https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/ocp-common/job/' + Jobname + '/' + run_id

    reponse = requests.get(url, verify=False)
    if reponse.status_code == 200:
        jenkins_html = reponse.text

        #####    use etree get junit_e2e.xml
        # etree = html.etree
        # element = etree.HTML(jenkins_html)
        # jenkinsxml = element.xpath('/html/body/div[4]/div[2]/table[1]/tbody/tr[1]/td[2]/table/tbody/tr/td[2]/a')

        #####    use bs4 get junit_e2e.xml
        soup = BeautifulSoup(jenkins_html, features="lxml")
        jenkins_xml = soup.find_all("table", class_="fileList")
        jen_xml_str = str(jenkins_xml)

        ###    use re find junit_e2e_.*.xml name
        file_name = re.compile('</td><td><a href="artifact/private/(.*?)">junit_e2e_.*?.xml</a></td><td class="fileSize">')
        xml_filename = ''.join(re.findall(file_name, jen_xml_str))

    ###     XML file exists. Continue execution
    if xml_filename != "":

        xmlurl = 'https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/ocp-common/job/' + Jobname + '/' + run_id + '/artifact/private/' + xml_filename
        print("check the url")
        print(xmlurl)

        console_url = 'https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/ocp-common/job/' + Jobname + '/' + run_id + '/consoleFull'

        r = requests.get(console_url, verify=False)

        with open("console.html", "wb") as code:
            code.write(r.content)
            code.close()

            open_con = open("console.html", "r")
            console_text = open_con.read()

            jenkins_result = []

            reg_build_version = re.compile(
                '<span class="timestamp"><b>.*?</b> </span><span style="display: none">.*?]</span> build_version: "(.*?)"')
            reg_LAUNCH_NAME = re.compile(
                '<span class="timestamp"><b>.*?</b> </span><span style="display: none">.*?</span> LAUNCH_NAME=(.*)')
            reg_PROFILE_NAME = re.compile(
                '<span class="timestamp"><b>.*?</b> </span><span style="display: none">.*?]</span> PROFILE_NAME=(.*)')

            build_version = re.findall(reg_build_version, console_text)
            launch_name = re.findall(reg_LAUNCH_NAME, console_text)
            profile_name = re.findall(reg_PROFILE_NAME, console_text)

            build_version_str = ''.join(build_version)
            launch_name_str = ''.join(launch_name)
            profile_name_str = ''.join(profile_name)

            sheet_name = which_sheet(build_version_str)


            profile_name_str_unescape = unescape(profile_name_str)

        if launch_name_str != '':

        #####   parser the xml
            try:
                r = requests.get(xmlurl, verify=False)
                with open("junit_e2e.xml", "wb") as code:
                    code.write(r.content)
                tree = ET.parse('junit_e2e.xml')

                testsuite = tree.getroot()

                for testcase in testsuite:
                    try:
                        if testcase[0].text != "":

                            testcase_info_dict = dict(testcase.attrib)
                            failure = str(testcase[0].text)
                            sysoutfull = str(testcase[1].text)
                            sysout = sysoutfull[:50000]

                            # testcase_info_dict is "{'name': '[sig-operators] OLM opm with podman Author:tbuskey-VMonly-High-30786-Bundle addition commutativity [Suite:openshift/conformance/parallel]', 'time': '35'}"
                            xml_result = []

                            if testcase_info_dict['name']:
                                testcase_info_dict['department'] = testcase_info_dict['name'].split(' ')[1]
                                tmp_author = testcase_info_dict['name'].split('Author:')[1]
                                if tmp_author:
                                    testcase_info_dict['author'] = tmp_author.split('-')[0]

                                department = testcase_info_dict['department']


                                #if department == 'OLM':
                                if department == 'OLM' or department == 'Operator_SDK' or department == 'Image_Registry':
                                #if department != '':

                                    # reg_owner = re.compile("{'name':.*?OLM.*?Author:(.*?)-.*?-\d{5}-.*?}")
                                    # reg_id = re.compile("{'name':.*?OLM.*?Author:.*?-(\d{5})-.*?}")
                                    # reg_describe = re.compile("{'name':.*?OLM.*?Author:.*?-.*?-\d{5}-(.*?)\s\[.*?}")
                                    reg_id = re.compile("\]\s.*?Author:.*?-(\d{5})-.*?")
                                    reg_describe = re.compile("\]\s.*?Author:.*?-\d{5}-(.*?)\s\[.*?")
                                    author = testcase_info_dict['author']


                                    # result = re.findall(reg,att)
                                    #owner = re.findall(reg_owner, testcase_info_dict)
                                    caseid = re.findall(reg_id, testcase_info_dict['name'])
                                    describe = re.findall(reg_describe, testcase_info_dict['name'])

                                    #ownerstr = ''.join(owner)
                                    caseidstr = ''.join(caseid)
                                    print(caseidstr)
                                    describestr = ''.join(describe)

                                    #####   save owner and caseid
                                    case_own = []
                                    case_own.insert(0, caseidstr)
                                    case_own.insert(1, describestr)
                                    case_own.insert(2, author)
                                    case_own.insert(3,department)

                                    xml_result.append(failure)
                                    xml_result.append(sysout)

                                    # jenkins_result.append(build_version)
                                    jenkins_result.append(profile_name_str_unescape)
                                    jenkins_result.append(launch_name_str)

                                    # the tool run date
                                    global strDate
                                    strDate= time.strftime('%Y/%m/%d', time.localtime(time.time()))
                                    run_tool_time = []
                                    run_tool_time.append('')
                                    run_tool_time.append(strDate)

                                    ###   Final list
                                    last_result = case_own + jenkins_result + run_tool_time + xml_result
                                    # print(last_result)

                                    del case_own[-4:]
                                    del xml_result[-1:]
                                    del jenkins_result[-2:]

                                    #worksheet.insert_row(xml_result,index=2, value_input_option='RAW')
                                    write_google_sheet(last_result, sheet_name)

                    except:
                        continue
            except:
                print('failed')

def which_sheet(build_version_str):
    ##   Put different sheets according to different versions
    if build_version_str == '4_9':
        sheet_name = Config[Jobname]['sheets'][-1]
    elif build_version_str == '4_8':
        sheet_name = Config[Jobname]['sheets'][-2]
    elif build_version_str == '4_7':
        sheet_name = Config[Jobname]['sheets'][-3]
    elif build_version_str == '4_6':
        sheet_name = Config[Jobname]['sheets'][-4]
    elif build_version_str == '4_5':
        sheet_name = Config[Jobname]['sheets'][-5]
    else:
        sheet_name = 'v4.5before_vm'
        print("sheet name wrong or not in yaml")
    return sheet_name


def write_google_sheet(result_list, sheet_name):
    scope = ['https://spreadsheets.google.com/feeds', 'https://www.googleapis.com/auth/drive']
    creds = ServiceAccountCredentials.from_json_keyfile_name(Config["google"]["key_file"], scope)
    client = gspread.authorize(creds)
    spreadsheet = client.open_by_url(Config["google"]["sheet_file"])

    worksheet = spreadsheet.worksheet(sheet_name)
    # print(result_list)

    # worksheet.insert_row([result_list],index=2,value_input_option='RAW')
    # worksheet.update('A7',[['Unknown, pleaes check', '', 'N 0', 'N 0', 'N 0', 'N 0', '', 0, 0, 0, 0, 0, '0%', '2021-08-03', 'http://virt-openshift-05.lab.eng.nay.redhat.com/buildcorp/nightly/[].html']])
    # worksheet.update('A8', [result_list])
    # worksheet.insert_row(['jitli'], index=2, value_input_option='RAW')
    # worksheet.update('A2','=HYPERLINK("jitli@redhat.com","email")')

    jenkins_num = (result_list[5])
    result_list[5] = ''
    # print(jenkins_num)

    worksheet.insert_row(result_list, index=2, value_input_option='RAW')
    # worksheet.update_acell('A2','=HYPERLINK("aslijitong@gmail.com","email")')
    # worksheet.update_acell('A2','=HYPERLINK("jitli@redhat.com",result_list[0])')
    worksheet.update_acell('F2',f'=HYPERLINK(CONCATENATE("https://mastern-jenkins-csb-openshift-qe.apps.ocp4.prod.psi.redhat.com/job/ocp-common/job/","{Jobname}","/",{run_id},"/testReport/"),"{jenkins_num}")')

    print("write row successfully")


########################################################################################################################################
if __name__ == '__main__':
###   Set the parameters
    parser = argparse.ArgumentParser()
    parser.add_argument('-c', '--configfile', type=str, required=True, help="The configuration file")
    parser.add_argument('-j', '--jobname', type=str, required=True, help="The Job Name")
    parser.add_argument('-n', '--number', type=int, default=10000, help="How many IDs to execute")
    # parser.add_argument('-r','--resultfile', type=str, required=True, help="The result file")
    args = parser.parse_args()
    Trace_Back = 50

    Jobname = args.jobname
    num = args.number
    if Jobname == "ginkgo-test-vm" or Jobname == "ginkgo-test":
        for i in range(num):

            with open(args.configfile) as f:
                Config = yaml.load(f, Loader=yaml.FullLoader)

                last_end_ID = Config[Jobname]["end_num"]
                run_id = str(last_end_ID + 1)

                ginkgo_google_report()

            ###    write this end ID into the yaml file
            with open(args.configfile, 'w') as f:
                Config[Jobname]["end_num"] = int(run_id)
                print(strDate)
                print(Config[Jobname]["end_num"])
                yaml.dump(Config, f)

                print("write ID in yaml successfully")
    else:
        print("The Job Name can only be ginkgo-test or ginkgo-test-vm")

