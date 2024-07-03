# GameServers-Updater

## 参数说明

- namespace：所处的命名空间
- gss-name：GameServerSet名称。支持填写多个gss，如 "pve,pvp"。当不填写时默认选择命名空间下全部gss
- timeout：执行超时时间。默认300s
- select-ids：选择处于Id列表中的GameServer。如 "1,2" 代表筛选id为1和2的gs
- select-opsState：选择opsState对应的GameServer。如 "None" 代表筛选opsState为None的gs
- select-networkDisabled：选择网络可用性为对应值的GameServer。如 "true" 代表筛选networkDisabled为true的gs
- select-not-container-image：选择容器镜像不为对应值的GameServer。如 "game/nginx:1.17" 代表筛选容器game镜像不为nginx:1.17的gs
- exp-opsState：希望更改opsState的对应值
- exp-networkDisabled：希望更改网络可用性的值

## 使用示例

可以通过创建K8s Job来使用，如

### example 1
希望将opsState为`None`，且容器`game`镜像**不**为`pvp:v2`的GameServer的opsState都标记为`WaitToBeDeleted`

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: example-1
  namespace: kruise-game-system
spec:
  template:
    spec:
      serviceAccountName: kruise-game-controller-manager
      containers:
      - name: updater
        image: registry.cn-beijing.aliyuncs.com/chrisliu95/gs-updater:v1.2
        command:
          - /updater
        args:
          - --gss-name=pvp
          - --namespace=default
          - --select-opsState=None
          - --select-not-container-image=minecraft/pvp:v2
          - --exp-opsState=WaitToBeDeleted
      restartPolicy: Never
EOF
```

### example 2
希望将id为[1,2,3]的GameServer的networkDisabled都标记为`true`

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: example-2
  namespace: kruise-game-system
spec:
  template:
    spec:
      serviceAccountName: kruise-game-controller-manager
      containers:
      - name: updater
        image: registry.cn-beijing.aliyuncs.com/chrisliu95/gs-updater:v1.2
        command:
          - /updater
        args:
          - --gss-name=pvp
          - --namespace=default
          - --select-ids=1,2,3
          - --exp-networkDisabled=true
      restartPolicy: Never
EOF
```
