# encoding: utf-8
#!/usr/bin/env python3
import re, os

# get the updated content
# ToDo: to infer the test case ID when the test case content updates, not the title update
content = os.popen('git show -m', 'r').read()
# content = '+   g.It("ConnectedOnly-High-37826-23170-20979-use an PullSecret for the private Catalog Source image [Serial]", func() {'
print "=======updated content========"
print content
print "=======updated content========"

# get the test case IDs
patternIt = re.compile('\+\s+g.It\(\".*?((\-\d+)+)')
itContent = patternIt.findall(content)
print "=======get the content via RE========"
print itContent
print "=======get the content via RE========"
if len(itContent) > 0:
    testcaseIDs = str(itContent[0][0]).strip("-").replace("-", "|")
    # run test case
    commands = './bin/extended-platform-tests run all --dry-run |grep -E '+ '"%s"' % (testcaseIDs) + ' |./bin/extended-platform-tests run -f -'
    print commands
    os.system(commands)
else:
    print "There is no Test Case found"

