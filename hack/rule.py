import re, os

content = os.popen('git show $(git rev-parse HEAD)', 'r').read()
print content

sigName = "sig-arch,sig-isc,sig-api-machinery,sig-auth,sig-apps,sig-cli,sig-scheduling,sig-etcd,sig-network,sig-network-edge,sig-storage,sig-openshift-logging,sig-devex, sig-builds,sig-ui,sig-instrumentation,sig-service-catalog,sig-operators,sig-operators,sig-imageregistry,sig-service-catalog,sig-hive,sig-windows,sig-testing,sig-scalability,sig-node,sig-node,sig-cluster-lifecycle,sig-node"

sigNameList = sigName.replace(' ', '').split(",")
print sigNameList

subTeam = "SDN,Storage,Developer Experience, User Interface, PerfScale, Service Development B, Node, Logging, Apiserver and Auth, Workloads, Metering, Cluster Observability, Quay/Quay.io, Cluster Infrastructure, Multi-Cluster, Cluster Operator, Azure, Network Edge, Etcd, Installer, Portfolio Integration, Service Development A, OLM, Operator SDK, App Migration, Windows Containers, Security and Compliance, KNI, Openshift Jenkins, RHV, ISV Operators, PSAP, Multi-Cluster-Networking, OTA"

subTeamList = subTeam.replace(' ', '').split(",")
print subTeamList

patternDescribe = re.compile('\+.*g.Describe\(\"(\[(.*)\]\s(.*)\")')
patternIt = re.compile('\+\s+g.It\(\".*\"')

itContent = patternIt.findall(content)
desContent = patternDescribe.findall(content)

print "find the Describe: %s" % (desContent,)
print "find the It: %s" % (itContent,)

findSig = False
findTeam = False

if len(desContent) > 0:
	# desContent[0][1] stores the sig name
	for sig in sigNameList:
		if sig in desContent[0][1]:
			findSig = True
			print sig
			break
	# desContent[0][2] stores the sub team
	for team in subTeamList:
		if team in desContent[0][2]:
			findTeam = True
			print team
			break
	
	if findSig and findTeam:
		print "the sig name: %s and sub team: %s exist" % (desContent[0][1], desContent[0][2])
	else:
		print "the sig or sub team name doesn't exist: %s" % (desContent[0],)
		raise Exception("please check the sig or sub team name above")

if len(itContent) > 0:
	mod = re.compile('g.It\((.*)-(\d+)-(.*)')
	itMode = mod.findall(itContent[0])
	if len(itMode) > 0:
		print "the title naming looks good!"
	else:
		print "Seems like it doesn't follow the naming rule: %s" % (itContent[0],)
		raise Exception("please check g.It title name above") 
