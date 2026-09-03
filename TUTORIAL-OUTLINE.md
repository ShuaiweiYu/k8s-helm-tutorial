# K8s + Helm 实战教程 · 60 分钟大纲

> **定位**：不讲 Docker 底层原理，不讲容器 namespace/cgroup。全程实践导向，重心在 K8s 对象模型和 Helm。
> **形式**：PPT 讲解 + 本 repo 现场演示交替进行。
> **听众预期**：会用 Docker，可能用过 docker-compose，没碰过或刚碰 K8s。

---

## 时间总表

| 时间 | 章节 | 核心目标 |
|---|---|---|
| 0–5 | 为什么需要编排 | 用 compose 建立锚点，然后打破它 |
| 5–17 | K8s 核心对象（手写 YAML） | Deployment / Service / 自愈 / 负载均衡 |
| 17–22 | **痛点爆发** | 让听众自己想要 Helm |
| 22–35 | Helm 基础 | 模板 + values + Release；两个 release + Ingress |
| 35–47 | 状态与配置 | 无状态→共享 DB→PVC；ConfigMap/Secret |
| 47–57 | **Helm 生命周期（高潮）** | upgrade → 翻车 → rollback |
| 57–60 | 收尾 | 生态与下一步 |

---

## 0–5 min · 为什么需要编排

### PPT 讲什么
- 一句话交代边界：**今天不讲 Docker 原理，默认大家会 `docker run`**。
- docker-compose 解决了什么：多容器、一份声明、一条命令起来。

### Demo（极速，不超过 2 分钟）
```bash
cd 00-compose
docker compose up -d
docker compose ps
curl localhost:8080
docker compose down
```

### 讲完 demo 立刻抛出四个问题（这是本节唯一的目的）
1. 容器半夜挂了，谁把它拉起来？
2. 流量涨了要跑 3 个副本，compose 怎么办？副本之间的流量谁分？
3. 要不停机地把 v1 换成 v2，怎么做？
4. 这台机器装不下了，要跑在 10 台机器上，怎么办？

> **过渡台词**：compose 是"在一台机器上把几个容器跑起来"，K8s 是"向集群描述你想要的最终状态，然后由它想办法一直维持住"。关键词：**声明式（declarative）**。

---

## 5–17 min · K8s 核心对象（手写 YAML）

### PPT 讲什么
- 一页架构简图带过：Control Plane（API Server / Scheduler / Controller Manager / etcd）+ Node（kubelet / kube-proxy）。**不要展开，30 秒。**
- 声明式 vs 命令式：`kubectl apply -f`（声明期望状态，可反复执行）vs `kubectl create`（一次性动作）。
- 对象层级：**Pod → ReplicaSet → Deployment**；**Service** 靠 **label selector** 找 Pod。
- 一句话点题：你写的是"我要 3 个副本"，不是"启动 3 个容器"。控制器负责让现实向期望收敛（reconcile loop）。

### Demo 分四步走

**第 1 步：Pod 不会自愈**
```bash
kubectl apply -f 01-k8s-raw/pod.yaml
kubectl get pods -o wide
kubectl describe pod hello
kubectl logs hello
kubectl delete pod hello
kubectl get pods          # 没了，不会自己回来
```
> 台词：Pod 是最小调度单位，但它是"一次性"的。生产里你几乎不会直接写 Pod。

**第 2 步：Deployment 会自愈**
```bash
kubectl apply -f 01-k8s-raw/deployment.yaml    # replicas: 2
kubectl get pods
kubectl delete pod <其中一个>
kubectl get pods          # 立刻有新 Pod 顶上，名字变了
```
> 分屏常驻 `watch kubectl get pods`，让听众亲眼看到 Pod 被重建。

**第 3 步：Service 负载均衡 —— 【要改的第 1 处，落点在这里】**

> ⚠️ **不要用"hello1 / hello2 两个不同应用"来演示负载均衡。**
> Service 负载均衡的是**同一个 Deployment 的多个副本**，靠 label selector 选 Pod。
> 两个内容不同的应用挂在一个 Service 后面属于硬凑，机制上是错的。
>
> **正确演法**：一个 Deployment、`replicas: 2`，应用页面上打印自己的 `HOSTNAME`（即 Pod 名）。

```bash
kubectl apply -f 01-k8s-raw/service.yaml
kubectl port-forward svc/hello 8080:80 &

# ⚠️ 不要用浏览器刷新！见文末"坑"第 1 条
for i in $(seq 30); do curl -s localhost:8080/ ; echo; done | sort | uniq -c
```
输出类似：
```
  14 hello-7d4b9c8f5-abcde
  16 hello-7d4b9c8f5-xyz12
```
> 台词：Service 是集群内的一个稳定虚拟 IP + DNS 名，它把流量分发到所有匹配 label 的 Pod。这是 **L4** 负载均衡。

**第 4 步：伸缩**
```bash
kubectl scale deployment hello --replicas=5
kubectl get pods
for i in $(seq 30); do curl -s localhost:8080/ ; echo; done | sort | uniq -c   # 现在是 5 个名字
```

### 本节小结（一个 demo 讲清三件事）
副本（replicas）· 自愈（controller reconcile）· 负载均衡（Service + label selector）。

---

## 17–22 min · 痛点爆发 —— 【要改的第 3 处，落点在这里】

> ⚠️ **这一节不能省。** 直接从 kubectl 跳到 Helm，听众只会觉得 Helm 是个"打包工具"，感受不到必要性。必须先制造痛苦。

### Demo：什么都不敲，只把文件摊在屏幕上
```bash
ls -1 01-k8s-raw/
# deployment.yaml
# service.yaml
# ingress.yaml
# configmap.yaml
# secret.yaml
```

### 然后当场发问
- 现在要上 **dev / staging / prod** 三套环境：副本数不同、镜像 tag 不同、域名不同、资源限制不同。**复制三份目录？**
- 升级一个镜像 tag，要在 3 个目录里改 3 个地方，改漏一个怎么办？
- 一个月后要下线这套东西，`kubectl delete` —— **你还记得当初 apply 过哪些文件吗？**
- 想把这套部署方案给同事复用，怎么交付？发压缩包？

### PPT：Helm 的四条价值（比"更系统化地管理 YAML"精确）
1. **模板化 + 参数化** —— 一套模板，`values.yaml` 决定环境差异。
2. **打包与版本化** —— chart 有自己的版本号，可以推到 OCI registry，像 npm 包一样分发。
3. **Release 生命周期** —— install / upgrade / **rollback** / uninstall 是一个**原子单位**。Helm 替你记住这个 release 包含哪些资源。
4. **依赖管理** —— `Chart.yaml` 里声明依赖，直接复用别人写好的 Postgres/Redis chart。

> 一句话：`kubectl apply` 是你自己记着做过什么；Helm 有 **release 状态**。

---

## 22–35 min · Helm 基础

### PPT 讲什么
- 三个核心概念：**Chart**（包）· **Values**（参数）· **Release**（一次安装的实例）。
- Chart 目录结构：
  ```
  chart/
    Chart.yaml        # 元数据：name / version / appVersion
    values.yaml       # 默认参数
    templates/        # Go template 渲染成 K8s YAML
      deployment.yaml
      service.yaml
      ingress.yaml
      _helpers.tpl    # 命名模板（复用 label / fullname）
    charts/           # 依赖的子 chart
  ```
- Go template 语法最小集：`{{ .Values.x }}`、`{{ .Release.Name }}`、`{{ include "chart.fullname" . }}`、`{{- if }}`、`{{- range }}`。

### Demo

**第 1 步：先看一眼官方脚手架，再说明我们用精简版**
```bash
helm create scratch && tree scratch    # 看一眼就删，官方模板太重，不适合教学
rm -rf scratch
tree 02-helm-basic/chart
```

**第 2 步：`helm template` —— 最重要的教学工具**
```bash
helm template hello ./02-helm-basic/chart
helm template hello ./02-helm-basic/chart --set replicaCount=5 | grep replicas
```
> 台词：这就是刚才手写的那些 YAML，只是现在它们是**算出来的**。这条命令不连集群、不改任何东西，是你调试模板的正道。

**第 3 步：安装**
```bash
helm install hello ./02-helm-basic/chart
helm list
kubectl get all -l app.kubernetes.io/instance=hello
helm get manifest hello        # Helm 记得这个 release 装了什么
```

**第 4 步：同一个 chart 装两次 —— 【要改的第 1 处的后半，hello1/hello2 在这里回归】**

> ⚠️ hello1 / hello2 这个点子是好的，只是**位置错了**。
> 它不该用来演示 Service 负载均衡，而该用来演示 **Ingress 路径路由 + Helm 的复用性**。
> 一石二鸟：两个 Service 恰好就是同一个 chart 传不同 values 装出来的两个 release。

```bash
helm install hello1 ./02-helm-basic/chart --set message="Hello 1"
helm install hello2 ./02-helm-basic/chart --set message="Hello 2"
helm list                      # 两个 release，一套模板
kubectl get svc
```

**第 5 步：Ingress 做 L7 路径路由**
```bash
kubectl apply -f 02-helm-basic/ingress.yaml
# /hello1 -> svc/hello1    /hello2 -> svc/hello2

curl -s localhost/hello1     # Hello 1 + pod 名
curl -s localhost/hello2     # Hello 2 + pod 名
```
> PPT 补一句：**Service 是 L4，Ingress 是 L7**。Ingress API 目前已冻结（不再加新特性），继任者是 **Gateway API**，知道有这回事即可。

---

## 35–47 min · 状态与配置

> 这是全场最有说服力的一段。三拍推进，每一拍都由上一拍的失败推出来。
> **【要改的第 2 处：不要给两个容器各配一个数据库，太绕。用内存计数器起手，10 秒出效果。】**

### 第 1 拍：无状态假设被打破（不需要数据库）

应用里放一个**内存计数器**，`replicas: 2`：
```bash
helm upgrade hello ./03-stateful/chart --set replicaCount=2
for i in $(seq 10); do curl -s localhost:8080/count ; echo; done
# 1 1 2 2 3 2 4 3 ...   数字乱跳
```
> 台词：每个 Pod 各数各的。**Pod 是可以随时被杀掉重建的，任何存在 Pod 内部的状态都会丢。** 这就是"无状态服务"的含义——状态必须放到外面去。

### 第 2 拍：共享数据库解决一致性
```bash
helm upgrade hello ./03-stateful/chart -f 03-stateful/values-with-db.yaml
for i in $(seq 10); do curl -s localhost:8080/count ; echo; done
# 1 2 3 4 5 6 ...   一致了
```
> PPT：应用通过 Service DNS 名访问数据库（`postgres.default.svc.cluster.local`），不需要知道 DB 的 IP。

### 第 3 拍：但数据还是会丢 → PersistentVolumeClaim
```bash
kubectl delete pod -l app=postgres
curl -s localhost:8080/count      # 从 1 重新开始 —— 数据没了！
```
> 台词：容器的文件系统跟着容器一起死。刚才那个 DB 用的是 `emptyDir`，Pod 一没就跟着没。

```bash
helm upgrade hello ./03-stateful/chart -f 03-stateful/values-with-pvc.yaml
kubectl get pvc,pv
# 灌一些数据，然后再杀一次
kubectl delete pod -l app=postgres
curl -s localhost:8080/count      # 数据还在
```
> PPT 收尾：
> - **PV / PVC**：PVC 是"我要一块 10G 的存储"，PV 是实际的那块盘，由 StorageClass 动态供应。
> - **StatefulSet**：为什么数据库通常用它而不是 Deployment —— 稳定的网络标识（`pg-0`、`pg-1`）、稳定的存储绑定、有序启停、`volumeClaimTemplates` 每个副本一块盘。
> - **诚实提醒（听众会很买账）**：生产环境很少手写 StatefulSet 跑数据库，一般走托管服务（RDS/CloudSQL）或成熟的 Operator。今天这么演是为了讲清 PVC 的机制。

### 配置：ConfigMap 与 Secret
```bash
kubectl get configmap hello-config -o yaml
kubectl get secret hello-db -o yaml
```
- **ConfigMap**：非敏感配置（日志级别、feature flag、DB 主机名）。
- **Secret**：敏感数据（密码、token、证书）。
- 两种注入方式：`envFrom` 注入环境变量 / 挂载成 volume 文件。

**Secret 并不加密**（30 秒，印象极深）：
```bash
kubectl get secret hello-db -o jsonpath='{.data.password}' | base64 -d; echo
```
> 台词：Secret 只是 **base64 编码，不是加密**。它相对 ConfigMap 的真正区别是 RBAC 可以单独管控、不会被随手打印到日志里、支持静态加密（但那是集群级别的可选配置）。真要管密钥，看 Sealed Secrets / External Secrets Operator / Vault。

**改了配置 Pod 不重启的坑**（引出后面的加分项 2）：
```bash
helm upgrade hello ./04-config/chart --set config.logLevel=debug
kubectl get pods       # AGE 没变，Pod 根本没重启，新配置没生效
```
> 精确说法：**`envFrom` 注入的环境变量不会热更新**；挂载成 volume 的文件**会**被 kubelet 更新（有延迟），但应用得自己 watch 文件变化。解法见"还能塞什么"第 2 条。

---

## 47–57 min · Helm 生命周期（高潮，务必留足时间）

### PPT 讲什么
- Release 有 **revision**，每次 upgrade 生成新版本，Helm 把历史存在集群里（Secret 形式）。
- Deployment 的滚动更新策略：`maxSurge` / `maxUnavailable`。
- 探针的作用：`readinessProbe` 决定"能不能接流量"，`livenessProbe` 决定"要不要重启我"。

### Demo：正常升级
```bash
# 开一个分屏
watch kubectl get pods

helm upgrade hello ./05-lifecycle/chart --set image.tag=v2
helm history hello
curl -s localhost:8080/          # v2 内容
```
> 让听众看着分屏里旧 Pod 逐个 Terminating、新 Pod 逐个 Running，**服务全程没断**。

### Demo：故意翻车
```bash
helm upgrade hello ./05-lifecycle/chart --set image.tag=v3-broken
kubectl get pods            # CrashLoopBackOff 一片飘红
kubectl logs -l app=hello --tail=20
curl -s localhost:8080/     # 还在返回 v2！readinessProbe 挡住了坏 Pod
```
> 关键教学点：**滚动更新 + readinessProbe 保护了你** —— 坏 Pod 没通过就绪检查，Service 不会把流量发给它。这就是为什么探针不是可选项。

### Demo：一键回滚（全场最好的一个瞬间）
```bash
helm history hello
helm rollback hello 2
kubectl get pods            # 秒回绿色
helm history hello          # 注意：rollback 会生成一个新的 revision，不是删掉历史
```

### 补一刀：让 Helm 自己回滚
```bash
helm upgrade hello ./05-lifecycle/chart --set image.tag=v3-broken --atomic --wait --timeout 60s
# 等待超时后 Helm 自动回滚，命令以非 0 退出 —— CI/CD 里就该这么写
```

---

## 57–60 min · 收尾

- **Kustomize vs Helm**：Kustomize 做 patch/overlay，无模板语言，K8s 原生集成（`kubectl apply -k`）；Helm 做模板 + 打包 + 生命周期。不是二选一，可以叠加用。
- **GitOps**：ArgoCD / Flux —— 把 `helm upgrade` 从"人在笔记本上敲"变成"Git 仓库即真相"。
- **Operator / CRD**：当 Helm 的"一次性部署"不够用，需要持续运维逻辑（备份、故障转移、扩容）时的下一步。
- 学习资源 + 本 repo 地址 + 每一步的 git tag 说明。

---
---

# 附录 A · 还能塞什么（按性价比排序，自己见缝插针）

> 按重要性排序。时间不够时**砍单顺序**：先砍 7（依赖管理），再砍 35–47 节里的 PVC 那一拍（保留"共享 DB 解决不一致"即可）。**别砍 rollback。**

### 1. `helm rollback` 翻车现场
已排进 47–57 节，全场最好的 demo，务必留足时间。故意 `--set image.tag=v3-broken`，`kubectl get pods -w` 里看着 CrashLoopBackOff 飘红，然后 `helm rollback hello 2` 瞬间绿回来。配合 `helm history` 看 revision 列表。

### 2. `checksum/config` 注解（Helm 的内行梗，用来收尾配置章节）
先演示痛点：改了 ConfigMap 的值 `helm upgrade` 之后 Pod **不会重启**，配置没生效。然后在 Deployment 的 pod template 上加：
```yaml
spec:
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```
配置一变 → 注解值变 → pod template 变 → 自动触发滚动更新。观众会"哦——"。

### 3. Secret 其实不加密
```bash
kubectl get secret hello-db -o jsonpath='{.data.password}' | base64 -d; echo
```
30 秒，印象极深。顺势提 Sealed Secrets / External Secrets / Vault。

### 4. values 分层与优先级
优先级从低到高：`chart/values.yaml` < `-f values-prod.yaml` < `--set`。
```bash
helm upgrade hello ./chart -f values-prod.yaml
helm upgrade hello ./chart -f values-prod.yaml --set replicaCount=10   # --set 赢
helm get values hello              # 看这个 release 最终用的是什么
```
演示同一个 chart 一条命令切 prod（副本数 1→3、加上资源限制、换域名）。

### 5. `--dry-run --debug` 与 `--atomic --wait`
```bash
helm upgrade hello ./chart --dry-run --debug     # 调试模板的正道，会连集群做校验
helm upgrade hello ./chart --atomic --wait --timeout 60s   # 失败自动回滚
```
后者和第 1 条的 rollback 连着讲，作为"生产里该怎么写"的收尾。

### 6. `_helpers.tpl` 命名模板 + 条件渲染
讲 chart 结构时顺手带出，不用单独占时间：
```yaml
{{- define "chart.fullname" -}}
{{ .Release.Name }}-{{ .Chart.Name }}
{{- end }}

{{- if .Values.ingress.enabled }}
...整个 Ingress 资源...
{{- end }}
```
配合 `helm template --set ingress.enabled=false` 看资源直接消失。

### 7. 依赖管理：把自己写的 Postgres 换成 chart 依赖
```yaml
# Chart.yaml
dependencies:
  - name: postgresql
    version: "x.y.z"        # 一定要锁死版本
    repository: "https://..."
```
```bash
helm dependency build ./chart       # ⚠️ 提前跑！把子 chart 落到 charts/ 目录
helm install hello ./chart
```
> ⚠️ **现场绝对不要联网拉 chart。** 提前 `helm dependency build`，把 `charts/` 目录连同 tgz 一起提交进 repo，并把镜像预加载进集群。

---
---

# 附录 B · 演示工程上的坑（最后的提醒，讲前必读）

> 这一页是保命用的。前面所有内容讲砸了都能补救，这几条踩了当场下不来台。

### 1. ⚠️ 浏览器刷新看不到 Pod 轮换 —— 最容易翻车的一条
kube-proxy 是**随机分发不是轮询**，而且浏览器 keep-alive 会复用 TCP 连接，你可能刷 10 次都是同一个 Pod，当场尴尬。**一律用命令行**：
```bash
for i in $(seq 30); do curl -s localhost:8080/ ; echo; done | sort | uniq -c
```
直接把分布打出来，比浏览器可信也更好看。（Ingress-nginx 到上游也有连接池，同理。）

### 2. 集群选型：用本机 kind，别用远程集群
本机当前的 `student` context 是远程集群，网络抖一下整场就废了。用 `kind`：配 `extraPortMappings` 暴露 80/443，装 ingress-nginx。本机目前**没装 kind 二进制**（`kubectl` / `helm` / `docker` 都有），讲之前先装上并跑通一次。

### 3. 所有镜像提前预加载，现场不依赖网络
```bash
kind load docker-image myapp:v1 myapp:v2 myapp:v3-broken postgres:16 --name demo
```
会场 wifi 是不可信的。包括 ingress-nginx 和依赖 chart 的镜像。

### 4. Makefile + git tag 做检查点（唯一可靠的保险）
```bash
make cluster      # 建集群 + 装 ingress + 预加载镜像
make reset        # 清空重来
make step-3       # 直接跳到第 3 节的状态
```
每一节一个 git tag。演示翻车时一条命令跳到下一节，**这是全场唯一的保险，一定要做**。

### 5. 分屏常驻 Pod 视图
一个分屏跑 `k9s` 或 `watch kubectl get pods`，让观众全程看到 Pod 的生灭，比讲十句话管用。

### 6. 终端准备
- 字号调大（至少 18–20pt），配色用高对比主题。
- `alias k=kubectl`（但 PPT 上写全称，方便听众抄）。
- `kubectl config set-context --current --namespace=demo`，别在 default 里跑。
- 提前 `kubectl config use-context kind-demo`，**别演到一半发现在往远程集群里 apply**。
- 关掉通知、勿扰模式、清空终端历史。

### 7. 演示应用要自己写，30 行足够
一个 Go / Node 小服务：
- 打印 `MESSAGE` 环境变量（区分 hello1 / hello2）
- 打印 `HOSTNAME`（Pod 名，负载均衡演示的核心）
- `/count` 计数器：有 DB 环境变量就走 DB，没有就走内存（状态一致性演示的核心）
- `/healthz` 就绪/存活探针
- 打三个 tag：`v1`、`v2`（改个颜色或文案）、`v3-broken`（**启动即退出**，用来演示回滚）

### 8. 时间一定会超
1 小时的现场 demo 从来没有不超时的。提前想好砍单顺序（见附录 A 开头），并在 35 分钟处设一个心理检查点：如果这时还没进 Helm 基础，直接砍掉 PVC 那一拍。
