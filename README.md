# nineinfra
The Nineinfra is the operator of the operators to manage the lifecycle of the data platforms.

## Description
Currently,the Nineinfra supports to manage the simple data house with the projects include kyuubi,metastore,postgresql,
minio and spark on k8s. In the future, more and more projects will gradually be added.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Installling the Nineinfra by helm
1. Add nineinfra helm repo

```sh
# helm repo add nineinfra https://nineinfra.github.io/nineinfra-charts/
# helm search repo nineinfra
NAME                                   CHART VERSION    APP VERSION    DESCRIPTION                        
nineinfra/cloudnative-pg        0.19.1           16.1.0         A Helm chart for cloudnative-pg    
nineinfra/kyuubi-operator       0.181.4          1.8.1          A Helm chart for Kyuubi Operator   
nineinfra/metastore-operator    0.313.3          3.1.3          A Helm chart for Metastore Operator
nineinfra/minio-directpv        4.0.8            4.0.8          A Helm chart for Minio DirectPV    
nineinfra/minio-operator        5.0.9            5.0.9          A Helm chart for Minio Operator    
nineinfra/nineinfra             0.4.4            0.4.4          A Helm chart for Nineinfra      
```

2. Install nineinfra by helm

```sh
# kubectl create namespace nineinfra
# helm install cloudnative-pg nineinfra/cloudnative-pg -n nineinfra
# helm install kyuubi-operator nineinfra/kyuubi-operator -n nineinfra
# helm install metastore-operator nineinfra/metastore-operator -n nineinfra
# helm install minio-directpv nineinfra/minio-directpv -n nineinfra
# helm install minio-operator nineinfra/minio-operator -n nineinfra
# helm install nineinfra nineinfra/nineinfra -n nineinfra
```

3. Check the nineinfra in the running

```sh
# kubectl get pod -n nineinfra
NAME                                             READY   STATUS    RESTARTS   AGE
cloudnative-pg-867997776f-jrtrg                  1/1     Running   0          3m16s
console-775674bdbf-vtgw5                         1/1     Running   0          115s
controller-576bc4d974-mw8vz                      3/3     Running   0          98s
controller-576bc4d974-s8bd6                      3/3     Running   0          98s
controller-576bc4d974-sdlf4                      3/3     Running   0          98s
kyuubi-operator-deployment-5884d5869b-bgsgn      1/1     Running   0          76s
metastore-operator-deployment-645cc7c84d-n96xq   1/1     Running   0          60s
minio-operator-68846c8cb8-qj6sz                  1/1     Running   0          115s
minio-operator-68846c8cb8-wfxpm                  1/1     Running   0          115s
node-server-b2kw8                                4/4     Running   0          98s
node-server-bt868                                4/4     Running   0          98s
node-server-l5gqb                                4/4     Running   0          98s
node-server-s8ghh                                4/4     Running   0          98s
```

4. Install kubectl plugin directpv 
Install krew according to https://krew.sigs.k8s.io/docs/user-guide/setup/install/
```sh
# kubectl krew install directpv
```

5. Create directpv drives on disks of the k8s nodes by kubectl plugin directpv

Discover the disk drives by the arg --drives which is according to your plan.
```sh
# kubectl directpv discover --nodes=nos-{13...16} --drives=vd{e...f}

 Discovered node 'nos-13' ✔
 Discovered node 'nos-14' ✔
 Discovered node 'nos-15' ✔
 Discovered node 'nos-16' ✔

┌─────────────────────┬────────┬───────┬──────────┬────────────┬──────┬───────────┬─────────────┐
│ ID                  │ NODE   │ DRIVE │ SIZE     │ FILESYSTEM │ MAKE │ AVAILABLE │ DESCRIPTION │
├─────────────────────┼────────┼───────┼──────────┼────────────┼──────┼───────────┼─────────────┤
│ 253:64$qminruVgo... │ nos-13 │ vde   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:80$Ch6FZ2Ogg... │ nos-13 │ vdf   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:64$E59EefLG5... │ nos-14 │ vde   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:80$ggm4ldlhL... │ nos-14 │ vdf   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:64$AMrZogHry... │ nos-15 │ vde   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:80$YWdsmkPJD... │ nos-15 │ vdf   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:64$IMnKcqGGA... │ nos-16 │ vde   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
│ 253:80$m4f/37aUP... │ nos-16 │ vdf   │ 1000 GiB │ xfs        │ -    │ YES       │ -           │
└─────────────────────┴────────┴───────┴──────────┴────────────┴──────┴───────────┴─────────────┘
Generated 'drives.yaml' successfully.
```
Add this drives to the directpv with the drives.yaml generated by the previous step.
```sh
# kubectl directpv init ./drives.yaml --dangerous
 Processed initialization request 'ab5160e6-3b1d-458b-9c79-b72a9fde8694' for node 'nos-14' ✔
 Processed initialization request '06e15423-67a6-47ae-bca3-94b6d33959f1' for node 'nos-15' ✔
 Processed initialization request '8d313b38-1ac0-4891-82a8-999087815962' for node 'nos-16' ✔
 Processed initialization request '326d2668-af14-4d1c-bdb9-676a8e833db6' for node 'nos-13' ✔


┌──────────────────────────────────────┬────────┬───────┬─────────┐
│ REQUEST_ID                           │ NODE   │ DRIVE │ MESSAGE │
├──────────────────────────────────────┼────────┼───────┼─────────┤
│ 326d2668-af14-4d1c-bdb9-676a8e833db6 │ nos-13 │ vde   │ Success │
│ 326d2668-af14-4d1c-bdb9-676a8e833db6 │ nos-13 │ vdf   │ Success │
│ ab5160e6-3b1d-458b-9c79-b72a9fde8694 │ nos-14 │ vde   │ Success │
│ ab5160e6-3b1d-458b-9c79-b72a9fde8694 │ nos-14 │ vdf   │ Success │
│ 06e15423-67a6-47ae-bca3-94b6d33959f1 │ nos-15 │ vde   │ Success │
│ 06e15423-67a6-47ae-bca3-94b6d33959f1 │ nos-15 │ vdf   │ Success │
│ 8d313b38-1ac0-4891-82a8-999087815962 │ nos-16 │ vde   │ Success │
│ 8d313b38-1ac0-4891-82a8-999087815962 │ nos-16 │ vdf   │ Success │
└──────────────────────────────────────┴────────┴───────┴─────────┘
```
### Creating the data house by the Nineinfra
You just need to specify only one parameter to create a data house.
The parameter dataVolume is the capacity of the data house, the unit is Gi.
```sh
# cat config/samples/nine_v1alpha1_ninecluster.yaml
apiVersion: nine.nineinfra.tech/v1alpha1
kind: NineCluster
metadata:
  labels:
    app.kubernetes.io/name: ninecluster
    app.kubernetes.io/instance: ninecluster-sample
  name: ninecluster-sample
spec:
  dataVolume: 32
```
Apply the yaml
```sh
# kubectl create namespace dwh
# kubectl apply -f config/samples/nine_v1alpha1_ninecluster.yaml
```
Check the data house in the running
```sh
# kubectl get pod -n dwh
NAME                                  READY   STATUS    RESTARTS   AGE
ninecluster-sample-nine-kyuubi-0      1/1     Running   0          60s
ninecluster-sample-nine-metastore-0   1/1     Running   0          62s
ninecluster-sample-nine-pg-1          1/1     Running   0          103s
ninecluster-sample-nine-pg-2          1/1     Running   0          76s
ninecluster-sample-nine-pg-3          1/1     Running   0          58s
ninecluster-sample-nine-ss-0-0        2/2     Running   0          104s
ninecluster-sample-nine-ss-0-1        2/2     Running   0          104s
ninecluster-sample-nine-ss-0-2        2/2     Running   0          104s
ninecluster-sample-nine-ss-0-3        2/2     Running   0          104s
```
### Operating the data house with sql
```sh
# kubectl get svc -n dwh |grep kyuubi
ninecluster-sample-nine-kyuubi       ClusterIP   10.99.90.150     <none>        10099/TCP,10009/TCP   3m13s


kubectl exec -it ninecluster-sample-nine-kyuubi-0 -n dwh -- bash
kyuubi@ninecluster-sample-nine-kyuubi-0:/opt/kyuubi$ cd bin
kyuubi@ninecluster-sample-nine-kyuubi-0:/opt/kyuubi/bin$ ./beeline
Warn: Not find kyuubi environment file /opt/kyuubi/conf/kyuubi-env.sh, using default ones...
Beeline version 1.8.0-SNAPSHOT by Apache Kyuubi
beeline>
beeline> !connect jdbc:hive2://ninecluster-sample-nine-kyuubi:10009

0: jdbc:hive2://ninecluster-sample-nine-kyuub> create database test;
0: jdbc:hive2://ninecluster-sample-nine-kyuub> use test;
0: jdbc:hive2://ninecluster-sample-nine-kyuub> create table test(id int,name string);
0: jdbc:hive2://ninecluster-sample-nine-kyuub> insert into test values (1,"nineinfra");
0: jdbc:hive2://ninecluster-sample-nine-kyuub> select * from test;
+-----+------------+
| id  |    name    |
+-----+------------+
| 1   | nineinfra  |
+-----+------------+
1 row selected (3.868 seconds)
```
### Uninstalling the Nineinfra
1. Delete the nineclusters
```sh
# kubectl delete -f config/samples/nine_v1alpha1_ninecluster.yaml -n dwh
ninecluster.nine.nineinfra.tech "ninecluster-sample" deleted
```
2. Delete residual PVCs
```sh
# for pvc in `kubectl get pvc -n dwh |grep directpv-min-io|awk -F ' ' '{print $1}'`;do kubectl delete pvc $pvc -n dwh;done
```
3. Delete the directpv drives
```sh
# kubectl directpv remove --all
```
4. Delete the Operators
```sh
# helm list -n nineinfra
NAME                    NAMESPACE       REVISION        UPDATED                                 STATUS          CHART                           APP VERSION
cloudnative-pg          nineinfra       1               2023-11-18 21:36:08.586923675 +0800 CST deployed        cloudnative-pg-0.19.1           1.21.1
kyuubi-operator         nineinfra       1               2023-11-18 14:18:23.345297231 +0800 CST deployed        kyuubi-operator-v0.18.1         0.18.1
metastore-operator      nineinfra       1               2023-11-18 14:27:03.694032911 +0800 CST deployed        metastore-operator-v0.313.3     0.313.3
minio-directpv          nineinfra       1               2023-11-18 21:15:34.758540459 +0800 CST deployed        minio-directpv-chart-v4.0.8     4.0.8
minio-operator          nineinfra       1               2023-11-18 20:32:02.839089348 +0800 CST deployed        minio-operator-chart-v5.0.9     5.0.9
nineinfra               nineinfra       1               2023-11-18 21:24:43.122424565 +0800 CST deployed        nineinfra-v0.4.4                0.4.4
# helm uninstall cloudnative-pg -n nineinfra
# helm uninstall kyuubi-operator -n nineinfra
# helm uninstall metastore-operator -n nineinfra
# helm uninstall minio-directpv -n nineinfra
# helm uninstall minio-operator -n nineinfra
# helm uninstall nineinfra -n nineinfra
```
## Contributing
Contributing is very welcome.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023 nineinfra.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

