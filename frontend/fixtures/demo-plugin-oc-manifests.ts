export const DemoPluginNamespace = {
  "apiVersion": "v1",
  "kind": "Namespace",
  "metadata": {
     "name": "console-demo-plugin"
  }
}

export const DemoPluginDeployment = {
"apiVersion": "apps/v1",
"kind": "Deployment",
"metadata": {
   "name": "console-demo-plugin",
   "namespace": "console-demo-plugin",
   "labels": {
      "app": "console-demo-plugin",
      "app.kubernetes.io/component": "console-demo-plugin",
      "app.kubernetes.io/instance": "console-demo-plugin",
      "app.kubernetes.io/part-of": "console-demo-plugin",
      "app.openshift.io/runtime-namespace": "console-demo-plugin"
   }
},
"spec": {
   "replicas": 1,
   "selector": {
      "matchLabels": {
         "app": "console-demo-plugin"
      }
   },
   "template": {
      "metadata": {
         "labels": {
            "app": "console-demo-plugin"
         }
      },
      "spec": {
         "containers": [
            {
               "name": "console-demo-plugin",
               "image": "PLUGIN_IMAGE",
               "ports": [
                  {
                     "containerPort": 9001,
                     "protocol": "TCP"
                  }
               ],
               "imagePullPolicy": "Always",
               "args": [
                  "--ssl",
                  "--cert=/var/serving-cert/tls.crt",
                  "--key=/var/serving-cert/tls.key"
               ],
               "volumeMounts": [
                  {
                     "name": "console-serving-cert",
                     "readOnly": true,
                     "mountPath": "/var/serving-cert"
                  }
               ]
            }
         ],
         "volumes": [
            {
               "name": "console-serving-cert",
               "secret": {
                  "secretName": "console-serving-cert",
                  "defaultMode": 420
               }
            }
         ],
         "restartPolicy": "Always",
         "dnsPolicy": "ClusterFirst"
      }
   },
   "strategy": {
      "type": "RollingUpdate",
      "rollingUpdate": {
         "maxUnavailable": "25%",
         "maxSurge": "25%"
      }
   }
  }
}
export const DemoPluginService= {
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
     "annotations": {
        "service.alpha.openshift.io/serving-cert-secret-name": "console-serving-cert"
     },
     "name": "console-demo-plugin",
     "namespace": "console-demo-plugin",
     "labels": {
        "app": "console-demo-plugin",
        "app.kubernetes.io/component": "console-demo-plugin",
        "app.kubernetes.io/instance": "console-demo-plugin",
        "app.kubernetes.io/part-of": "console-demo-plugin"
     }
  },
  "spec": {
     "ports": [
        {
           "name": "9001-tcp",
           "protocol": "TCP",
           "port": 9001,
           "targetPort": 9001
        }
     ],
     "selector": {
        "app": "console-demo-plugin"
     },
     "type": "ClusterIP",
     "sessionAffinity": "None"
  }
} 
export const DemoPluginConsolePlugin = {
  "apiVersion": "console.openshift.io/v1alpha1",
  "kind": "ConsolePlugin",
  "metadata": {
     "name": "console-demo-plugin"
  },
  "spec": {
     "displayName": "OpenShift Console Demo Plugin",
     "service": {
        "name": "console-demo-plugin",
        "namespace": "console-demo-plugin",
        "port": 9001,
        "basePath": "/"
     },
     "proxy": [
        {
           "type": "Service",
           "alias": "demoplugin",
           "name": "thanos-querier",
           "service": {
              "name": "thanos-querier",
              "namespace": "openshift-monitoring",
              "port": 9091,
              "authorize": true
           }
        }
     ]
  }
}
