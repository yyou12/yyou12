# encoding: utf-8
#!/usr/bin/env python3
import re, os

# get the updated content
# ToDo: to infer the test case ID when the test case content updates, not the title update
# `git show master..` can get all the updated commits content
content = os.popen('git show master..', 'r').read()
# content = '''
# +    g.It("Medium-34883-SDK stamp on Operator bundle image", func() {
# +   g.It("ConnectedOnly-High-37826-Low-23170-Medium-20979-High-37442-use an PullSecret for the private Catalog Source image [Serial]", func() {
# '''

print ("=======updated content========")
print (content)
print ("=======updated content========")

# get the test case IDs
patternIt = re.compile('\+\s+g.It\(\".*?(([A-Za-z]+\-\d+\-)+)')
itContent = patternIt.findall(content)
print ("=======get the content via RE========")
print (itContent)
print ("=======get the content via RE========")
if len(itContent) > 0 and len(itContent[0]) > 0:
    testcaseIDs = filter(lambda ch: ch in '0123456789-', str(itContent[0][0]).strip("-"))
    testcaseIDs = str(testcaseIDs).strip("-").replace("--", "|")  
    # run test case
    commands = './bin/extended-platform-tests run all --dry-run |grep -E '+ '"%s"' % (testcaseIDs) + ' |./bin/extended-platform-tests run -f -'
    print (commands)
    os.system(commands)
else:
    print ("There is no Test Case found")    
