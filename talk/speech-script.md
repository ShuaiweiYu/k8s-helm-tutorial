# 发言稿 · K8s + Helm 实战分享（全程互动 live demo）

> 这份文件是**练习用的完整讲稿**，逐字写出来是为了让你在家念顺。上台时不要念它，台上用 [`cue-outline.md`](./cue-outline.md)。
>
> 标记约定：
>
> - **【说】** 台词，可以逐字念
> - **【敲】** 你在终端里敲的命令
> - **【互动】** 听众要做的动作，念出来，等他们做完再往下
> - **【屏】** 屏幕上此刻应该是什么
>
> 全场只有一个入口地址，写在白板上，整场不变：`http://<你的局域网 IP>:8080`
> 下文一律写成 `http://IP:8080`，练习时替换成 `ipconfig getifaddr en0` 的结果。
>
> 文末还有[附录：理论深挖卡](#附录理论深挖卡)，提问环节和听众追问时用，不进主线。

---

## 开场：先把所有人接进来

【屏】
- 幻灯片上准备好IP地址：`http://IP:8080`。
- 终端分屏
  - 左：命令行
  - 右上：`watch kubectl get pods`
  - 右下：ingress 访问日志

【说】
- 今天一个小时，讲 Kubernetes 和 Helm。
- 不讲 Docker 的底层原理。
-**我的笔记本现在就是一台服务器**，在demo中大家就访问我的电脑来看会发生什么。

【互动 0】
连公司的 WiFi，名字是 `Reply`。

- 等待大家准备Terminal 和 准备IP地址

---

## 从 docker compose 出发

【说】
- Go 小程序
  - 打印 `MESSAGE` 环境变量
  - 打印自己的 hostname
  - 还有一个 `/count` 计数器。

【说】
- 为什么需要Docker？
  - **"在我机器上明明是好的"**：你本机装的 Go 版本、系统库、环境变量，跟服务器上的不是同一套，代码一到线上就炸。
  - Docker 把应用**连同它运行所需要的一切**（运行时、依赖、文件系统）一起打进一个镜像，镜像在哪儿跑结果都一样，这叫 **build once, run anywhere**。
  - **隔离**。同一台机器上跑十个应用，Docker 让它们各自以为自己独占一台机器，互相看不见对方的文件、进程、端口，不会互相污染。

- 如何dockerize一个应用

【屏】展示 `app/Dockerfile`

【说】
- Dockerfile 里有两个 `FROM`，**多阶段构建（multi-stage build）**
- **第一阶段（`build`）**：从 `golang:1.22-alpine` 出发，这个镜像里装着完整的 Go 工具链——编译器、标准库、`go mod` 这些。它把源码编译成一个**静态二进制**。
- **第二阶段（运行阶段）**：从 `gcr.io/distroless/static-debian12` 重新出发——这个基础镜像里**没有 shell、没有包管理器、连 `ls` 都没有**，只有跑一个静态二进制所需要的最基本的东西。`COPY --from=build` 只把上一阶段编译出来的那一个二进制文件拷过来，Go 工具链、源码、中间产物全部留在第一阶段，不会进最终镜像。

- 多阶段构建的好处：
  -  **体积小**：最终镜像里没有编译器、没有源码、没有用不上的系统工具，一个几百 MB 的构建环境最后只剩几 MB 到几十 MB。
  -  **攻击面小**：没有 shell 意味着就算有人在你的应用里找到一个漏洞能执行命令，这个环境里**根本没有命令可执行**——没有 `sh`，没有 `curl`，没有任何能被拿来做下一步的工具。

【敲】

```bash
docker run -d --name hello -p 8080:8080 -e MESSAGE="Hello from docker run" hello:v1
```

【说】
先看这条命令里的几个参数：

- **`-d`（detach）**：后台运行，不占着这个终端窗口，命令立刻返回。
- **`--name hello`**：给容器起个名字。不给的话 Docker 会随机分配一个，后面 `docker logs`、`docker stop` 都得用这个名字或者一串容器 ID 来指，取个名字方便操作。
- **`-p 8080:8080`**：端口映射，**冒号左边是宿主机端口，右边是容器内部端口**。容器里的进程监听的是它自己网络命名空间里的 8080，外面本来看不见；这条参数在宿主机上开一个 8080，把流量转发进容器的 8080。
- **`-e MESSAGE="Hello from docker run"`**：设置容器内的环境变量。

【敲】

```bash
curl localhost:8080
```

【说】
- 输出是 `Hello from docker run from d8f4df4bdc95 (version v1)`。
- 程序会打印 `os.Hostname()`。**Docker 默认把容器短 ID 设成它的主机名**，所以查出来就是这串 ID。

- 一个应用通常不止一个容器，还要配数据库、缓存。
- 如果给我们的 hello 应用配置数据库手写 `docker run` 是这样：

【屏】

```bash
docker network create hello-net

docker run -d --name hello-postgres \
  --network hello-net \
  -e POSTGRES_PASSWORD=postgres \
  -v pgdata:/var/lib/postgresql/data \
  -p 5432:5432 \
  postgres:16

docker run -d --name hello \
  --network hello-net \
  -e DB_HOST=hello-postgres \
  -e DB_PASSWORD=postgres \
  -p 8080:8080 \
  hello:v1
```

【说】
- 提一嘴说 docker run 非常麻烦，容易出错
- 而使用 compose 就可以加速我们的开发流程

【屏】展示 compose 文件 `00-compose/docker-compose.yaml`

【说 + 屏】
- **`hello:`**：服务名，compose 靠这个名字管它，等价于刚才的 `--name hello`。
- **`build: ../app`**：本地没有这个镜像的话，就用 `../app` 目录下的 Dockerfile 现场构建。
- **`ports: - "8080:8080"`**：等价于 `-p 8080:8080`，宿主:容器。
- **`environment: MESSAGE: "Hello from compose"`**：等价于 `-e MESSAGE="Hello from compose"`。
- **`depends_on: postgres: condition: service_healthy`**：`hello` 等 `postgres` 的 `healthcheck` 通过了才启动，不是等它"起了"，是等它"能用了"——避免 app 先起、数据库还没准备好连接就失败。


【敲】

```bash
cd 00-compose
docker compose up -d
docker compose ps
curl localhost:8080
docker compose down
```

【说】
到这里为止都很顺。现在我问四个问题：

1. 这个容器半夜因为一个空指针挂了，**谁把它拉起来**？
2. 流量涨了，要跑三个副本，compose 怎么办？三个副本之间的**流量谁来分**？
3. 要**不停机**地把 v1 换成 v2，怎么做？
4. 这一台机器装不下了，要跑在十台机器上，怎么办？

compose 管的是"在一台机器上把几个容器跑起来"，`docker compose up` 这条命令跑完就结束了，**背后没有任何人继续替你盯着**。而我们真正想要的是：我描述一个我期望的样子，然后有人一直帮我维持住这个样子。这件事叫**容器编排**，Kubernetes 就是现在最主流的那一套。

---

## K8s 是什么：声明式与控制循环

【屏】架构图（Control Plane + Node）。

【说】
- K8s 分两部分。
- **Control Plane** 是大脑：
  - API Server 是唯一入口，所有组件之间从不直连，全都只跟它说话；
  - etcd 存集群的全部状态；
  - Scheduler 决定新 Pod 放到哪台机器；
  - Controller Manager **不停地对比"期望状态"和"现实状态"，发现不一样就去补齐**，这个动作叫 reconcile。
- **Node** 是干活的机器：
  - kubelet 负责在本机把容器拉起来，
  - kube-proxy 负责网络转发。

【屏】展示 一段模版manifest

我们写给 K8s 的每一份 YAML 都叫 **manifest**，由四部分组成：

- `apiVersion`：标注这个对象该用哪个 API 哪个版本来解析
- `kind`：声明这是哪种对象；
- `metadata`：这个对象的身份信息，名字、标签、所在 namespace 都写在这里，K8s 靠它认出"这是谁"；
- `spec`：描述"我期望的状态是什么样"。

---

## Pod 与 Deployment

【屏】展示 `01-k8s-raw/pod.yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hello
  labels:
    app: hello-standalone
spec:
  containers:
    - name: hello
      image: hello:v1
      imagePullPolicy: IfNotPresent
      ports:
        - containerPort: 8080
```

【说】
- `kind: Pod`：Pod 是 K8s 里最小的单位，作用是用来跑一个容器。
- `spec` 里描述"我要一个跑 `hello:v1` 这个镜像的容器"。

【敲】

```bash
kubectl create -f 01-k8s-raw/pod.yaml
kubectl create -f 01-k8s-raw/pod.yaml
```

【说】
第二条直接报错 `AlreadyExists`。`create` 是**命令式**的——它描述一个动作，"创建"这件事本身第二次没有意义，K8s 老老实实告诉你"这个我已经建过了"。

【敲】

```bash
kubectl delete pod hello
kubectl apply -f 01-k8s-raw/pod.yaml
kubectl apply -f 01-k8s-raw/pod.yaml
```

【说】
这次两条都没报错，结果也一样。`apply` 是**声明式**的——它提交的不是"创建"这个动作，而是"我期待集群变成这个样子"，集群自己去比对该创建还是该修改。

【屏】展示声明式和命令式的对比图。

【敲】

```bash
kubectl get pods -o wide
kubectl logs hello
```

【说】
`READY 1/1`、`STATUS Running`，它跑起来了，现在我删掉它，看一下会发生什么：

【敲】

```bash
kubectl delete pod hello
kubectl get pods
```

【说】
- Pod消失
- Pod没有自动创建
- 这就是 Pod 的性质：**它是一次性的**。
  - K8s 按描述创建了一个 Pod
  - 没有机制负责让这个 Pod 一直活着。
  - 进程退出、节点重启、手滑删掉，它就真的消失了。
  - 所以生产环境里几乎没人直接手写 Pod。

【屏】展示 `01-k8s-raw/deployment.yaml`

我们要的不是"创建一个 Pod"这个动作，而是"这里应该一直有 N 个 Pod"这个约定。这就是 Deployment。

【敲】

```bash
kubectl apply -f 01-k8s-raw/deployment.yaml
kubectl get deployments,replicasets,pods
```

【说】
- 两个 Pod 都 `Running` 了。
- 杀掉一个
- 让观众看右上角 `watch` 窗口。

【敲】

```bash
kubectl delete pod hello-6b85969876-49nxs
```

【说】
- 删除命令刚返回，一个新 Pod 已经在 `ContainerCreating` 了
- 几秒之后回到 2 个
- 而且名字变了
  - 随机后缀不一样
  - IP 也不一样。

- K8s **新建了一个** Pod
- ReplicaSet 只数数：期望 2 个、现实 1 个、差 1 个、补一个。这就是刚才说的 reconcile。
- 删 Pod 这个动作改变了"现实"，控制器发现现实和 `replicas: 2` 这个"期望"对不上，于是自动补齐。

## Service 与负载均衡

【说】
- Pod 如果一旦被重启，IP 随机分配，别人要怎么找到这组 Pod ？
- 同时怎么把流量分摊到多个副本上？
- Service 负责这两件：**service discovery**（怎么找到）和**负载均衡**（怎么分流量）

【屏】`01-k8s-raw/service.yaml`

【说】
整份 YAML 里最重要的是这三行：

```yaml
spec:
  selector:
    app: hello
```

- "**凡是带着 `app: hello` 这个标签的 Pod，都算它要转发流量的目标**"。

【敲】

```bash
kubectl apply -f 01-k8s-raw/service.yaml
kubectl get endpointslices -l kubernetes.io/service-name=hello
```

【说】
- 这份名单就是 Service 当前认下的转发目标
- 控制器根据标签实时维护的
  - Pod 没了就从名单里划掉。
  - 新 Pod 起来就自动加进去。

【互动 1】
- 观众输入指令

```bash
curl http://IP:8080
```

- 观众看到 `Hello from ... (version v1)`
- 对比后缀的名称
- 看到一共有两个后缀

【互动 2】
- 观众输入指令，**访问 5 次**

```bash
for i in $(seq 5); do curl -s http://IP:8080/ ; done
```

- 观察是否两个后缀交替出现
- kube-proxy 默认是**按概率随机挑**，不是严格轮询。

【互动 3】

明显，有这么多人访问我们的服务，我们现在需要扩容。

```bash
kubectl scale deployment hello --replicas=5
```

- 三个的新 Pod 被建立了。
- 观众输入指令，**访问 5 次**


## 痛点爆发：为什么需要 Helm

【说】
到这儿为止一切都很顺，因为我只有一个应用、一套环境、三份文件（`pod.yaml`、`deployment.yaml`、`service.yaml`）。现在设想一下真要把这套东西送上线：日志级别这类配置得用 **ConfigMap**，数据库密码这类敏感信息得用 **Secret**，想让外面用域名访问得用 **Ingress**——每多一种真实需求，就多一份 YAML。现在我问四个问题：

**第一**，这套东西真上线，至少要 dev、staging、prod 三套环境：副本数不同、镜像 tag 不同、域名不同、prod 还要加资源限制。怎么办？**复制三份目录出来手改？**

**第二**，假设你真的复制了三份。每次发版都要在三个目录里改三处 image tag，再 apply 三次。**改漏一处不会报错**——那套环境只是安静地继续跑着旧版本，直到有人发现。

**第三**，一个月后这套东西要下线。`kubectl delete` 的时候，**你还记得当初 apply 过哪些文件吗**？

**第四**，同事想复用你这套部署方案，你怎么交付给他？发个压缩包？

`kubectl apply` 解决的是"怎么描述一个对象"，它没有解决"怎么管理一堆对象，还要在多个环境里各来一份"。**一堆 YAML 文件**这个交付形态，没有参数、没有版本号、也没有人替你记账。

Helm 就是来补这三样的。

---

## Helm 基础：Chart、Values、Release

【说】
**Helm 是 Kubernetes 的包管理器**。拿 npm 和 apt 类比最准：npm 管的是"一个软件包怎么被描述、参数化、安装、升级、卸载"，Helm 管的是"**一组 K8s 对象**怎么被描述、参数化、安装、升级、卸载"。

它是一个跑在我笔记本上的命令行工具，把模板渲染成普通的 K8s YAML，再通过 API Server 提交上去。**集群那一侧收到的东西，跟 `kubectl apply` 提交的没有任何区别**——这句话很重要，等会儿会验证。

三个核心概念：

- **Chart** 是包：一个目录，里面是参数化的模板加一份默认参数。它有自己的版本号，可以打包、推到 registry。
- **Values** 是这一次要填进去的参数：副本数几个、镜像哪个 tag、要不要开 Ingress。
- **Release** 是 chart 配上一份 values、在集群里真正装出来的那个实例。它有名字，有修订号 revision，第一次安装是 1，之后每次 upgrade 加一。

【屏】`tree 02-helm-basic/chart`

【说】
目录结构是固定的，Helm 靠**文件名和目录名**识别：`Chart.yaml` 是元数据，`values.yaml` 是默认参数，`templates/` 里每个 yaml 都会被渲染成 manifest 提交，下划线开头的 `_helpers.tpl` 不会被提交，只是存可复用的片段。

模板长这样——跟刚才手写的 deployment 骨架**一模一样**，差别只在被 `{{ }}` 换掉的那几处：

```yaml
spec:
  replicas: {{ .Values.replicaCount }}
  ...
      containers:
        - name: {{ .Chart.Name }}
          image: '{{ .Values.image.repository }}:{{ .Values.image.tag }}'
```

`.Values` 来自 values.yaml，`.Chart` 来自 Chart.yaml，`.Release.Name` 是这次安装的名字——这个值渲染的时候才知道，所以不可能写在文件里。

现在讲**今天最值得你记住的一条命令**：

【敲】

```bash
helm template hello ./02-helm-basic/chart
helm template hello ./02-helm-basic/chart --set replicaCount=5 | grep replicas
```

【说】
`helm template` 在本地把模板和 values 拼成完整的 manifest 打印出来，**不连集群、不改任何东西**。你们看，打印出来的就是刚才我手写的那些 YAML，只是现在每个值都是**算出来的**。调模板的时候永远先用它，别拿集群当试验田。

第二条命令 `--set replicaCount=5`，文件一个字没改，参数被覆盖了。覆盖参数的优先级从低到高是：`values.yaml` < `-f 某份文件` < `--set`，而且是**逐字段合并**，不是整份替换。

好，现在真的装上去。先把手写的那套删掉，因为 Helm 拒绝接管不是自己创建的对象：

【敲】

```bash
kubectl delete -f 01-k8s-raw/deployment.yaml -f 01-k8s-raw/service.yaml
```

【说】
（你们会短暂看到 503，正常，马上回来。）

回到开头那个问题：三套环境要不要复制三份目录？Helm 的答案是：**目录只有一份，环境差异各写一份 values 文件**。仓库里两份 values，每份只写了跟默认值不一样的字段：

```yaml
# values-dev.yaml
replicaCount: 1
message: 'Hello dev'
```

```yaml
# values-prod.yaml
replicaCount: 3
message: 'Hello prod'
```

【敲】

```bash
helm install dev  ./02-helm-basic/chart -f 02-helm-basic/values-dev.yaml
helm install prod ./02-helm-basic/chart -f 02-helm-basic/values-prod.yaml
helm list
```

【互动 4】
**麻烦大家分一下工**：左边这半屋子的人敲这条，右边这半把 `/dev` 换成 `/prod`：

```bash
for i in $(seq 5); do curl -s http://IP:8080/dev ; done
```

左边念一下看到什么？（`Hello dev from dev-xxx`）右边呢？（`Hello prod from prod-xxx`）
再问左边：你那 5 行里几个名字？（**一个**，dev 只有 1 个副本）右边呢？（**三个**，prod 是 3 个副本）

【说】
一份模板、两份 values、两个互不干扰的环境，副本数还不一样。如果是手写 YAML，这就是两个目录、两次 apply、以及以后两倍的维护量。

顺便把**分流**这件事讲了：你们刚才访问的两条路径落到两个不同的服务上，靠的是 **Ingress**。它和 Service 的分工是：**Service 工作在 L4**，处理 IP 和端口，把流量摊给一组 Pod；**Ingress 工作在 L7**，看 HTTP 的 host 和 path，按规则转给不同的 Service。注意 Ingress 对象本身只是一份规则，真正干活的是集群里的控制器，我这里装的是 ingress-nginx。

（顺嘴一提：Ingress API 已经冻结不再加新特性了，继任者是 Gateway API，知道有这回事就行。）

最后是 Helm 相对于 kubectl 最实在的那一条：**它替你记账**。

【敲】

```bash
helm get manifest dev | head -20
kubectl get secret -l owner=helm
```

【说】
`helm get manifest` 读的是安装当时存下来的记录——哪怕我现在把整个 chart 目录删掉，它照样能告诉你这个 release 装了哪些对象。这份记录物理上就是集群里的一个 Secret，名字是 `sh.helm.release.v1.dev.v1`，末尾的 v1 是 revision。每 upgrade 一次多一个，这就是**回滚能实现的物理基础**。

顺便三个实际的后果，都是从"记录就是个 Secret"直接推出来的：

- **release 是有 namespace 的。** 记录存在哪个 namespace，release 就属于哪个 namespace，所以不同 namespace 里可以有同名的 release，`helm list` 默认也只列当前 namespace。
- **历史不是无限的**，默认只留最近 10 个 revision，再老的会被丢掉。你不可能回滚到半年前。
- **这个 Secret 里存的是完整 manifest 的压缩内容**，所以 chart 特别大的时候，upgrade 会撞上 etcd 单个对象 1MB 的上限——这是大 chart 用户会遇到的一个真实报错。

所以卸载不用你回忆：

【敲】

```bash
helm uninstall dev prod
```

【说】
Deployment、ReplicaSet、Pod、Service 一起消失，一条命令，不会漏。

---

## 状态与配置

【说】
刚才那个应用有一个没说破的前提：**它不记任何东西**。现在把这个前提打破。应用还有一个 `/count` 路由，访问一次加一，返回当前值。

【敲】

```bash
helm install hello ./03-stateful/chart
```

【互动 5】
所有人把地址改成 `/count`，**敲一次，只敲一次**：

```bash
curl -s http://IP:8080/count
```

现在**从这边开始，每个人把自己拿到的数字喊出来**……（听十来个）
听出问题了吗？**有重复。** 有两个人拿到 3，有两个人拿到 7。

【说】
原因是每次请求随机落到两个 Pod 之一，而这个计数器存在**Pod 自己的内存里**，两个 Pod **各数各的**，互相看不见。

更糟的是这些数字随时归零。Pod 不是你精心呵护的那台机器，它是控制器随时可以杀掉重建的一次性容器：升级镜像换 Pod、节点维护驱逐 Pod、探针失败重启容器。**存在 Pod 内部的状态，活不过这个 Pod。** 这就是"无状态服务"这个词的真正含义——不是应用没有状态，而是**状态必须放到 Pod 外面去**。

放哪儿？先放数据库。chart 里早就留好了位置，一个开关的事：

```yaml
# values-with-db.yaml
db:
  enabled: true
```

【敲】

```bash
helm upgrade hello ./03-stateful/chart -f 03-stateful/values-with-db.yaml
kubectl get pods,services
```

【互动 6】
多出来一个 Postgres 的 Pod 和一个 Service。现在**再来一次，每人敲一次 `/count`，再喊**……
（听十来个）这次**没有重复了**，数字是连着的。哪个 Pod 接到请求都不影响结果，因为数据已经不在 Pod 里了。

【说】
补一句机制：应用连数据库用的地址是 `hello-postgres`，**Service 的名字直接就是域名**。集群里的 CoreDNS 会把它解析到 ClusterIP。应用之间互相调用只要知道对方的 Service 名就够了，既不用记 IP，也不用自己做服务发现。

但是——数据库自己也跑在一个 Pod 里。我现在把它杀了：

【敲】

```bash
kubectl delete pod -l app=postgres
```

（等新 Pod 起来）

【互动】大家再敲一次 `/count`。

【说】
报错了，`relation "counter" does not exist`，500。不是从 1 重新开始，是**连表都没了**。因为新 Pod 拿到的是一块**全新的空盘**：这个 Postgres 挂的卷是 `emptyDir`，一块跟着 Pod 走的临时目录，**Pod 一删，它跟着一起消失**。

要让数据活得比 Pod 久，就得用 **PVC，PersistentVolumeClaim，持久卷声明**。名字已经说明了一切：它是一份**申请**——"我要一块 1Gi、读写模式 ReadWriteOnce 的存储"，由集群去找一块盘来满足它。盘本身是另一个对象 PersistentVolume，**它的生命周期跟 Pod 无关**：Pod 被删，盘还在，新 Pod 起来接着挂上去。

这里的设计又是同一个套路，**把"要什么"和"谁来提供"拆开**：

- **PVC 是需求**，应用开发者写，它只说"我要多大、什么读写模式"，**完全不关心底下是 AWS 的 EBS、是 NFS、还是这台笔记本上的一个目录**。
- **PV 是供给**，是真实的那块盘。
- **StorageClass 是中间的自动化**：与其让管理员预先手工切好一堆盘等着，不如声明一个"档次"（快的 SSD、便宜的机械盘），PVC 一提交，**由它现场去云厂商那儿开一块出来**，这叫动态供应。我们这儿用的是 minikube 默认的 `standard`。

所以同一份 chart 能在笔记本和生产集群上都跑起来，靠的就是这层解耦——**换集群的时候变的是 StorageClass，不是你的应用**。

两个必须知道的点：

- **accessMode 里的 ReadWriteOnce 是"单节点"可读写，不是"单 Pod"**。同一个节点上的多个 Pod 其实可以一起挂。真要多节点同时读写得是 ReadWriteMany，而那要求底层存储支持（NFS、CephFS 这类），**云厂商的块存储基本都不支持**。这就是为什么你不能简单地把数据库 `replicas: 3`——三个副本会被调度到三个节点上，抢同一块只能挂一处的盘。
- **删掉 PVC 之后盘会不会跟着没**，取决于 PV 的 reclaim policy。动态供应出来的默认是 `Delete`，**也就是说 `helm uninstall` 有可能把你的数据一起删掉**。生产上给数据库那块盘一定要确认这个策略。

第三份 values 就是把这个开关也打开：

【敲】

```bash
helm upgrade hello ./03-stateful/chart -f 03-stateful/values-with-pvc.yaml
kubectl get persistentvolumeclaims,persistentvolumes
kubectl rollout restart deployment/hello
```

【互动】大家敲几次 `/count`，数字从 1 开始往上走。
现在我**再杀一次数据库**：

```bash
kubectl delete pod -l app=postgres
```

（等 Pod 起来）再敲——**数字接着上次往下数**。新 Pod 挂回了同一块盘，数据还在。

【说】
一句诚实提醒，说了大家会更信：**上面这套是为了把 PVC 讲清楚，不是生产推荐做法。** 真要在集群里跑数据库，要么用 **StatefulSet**（它给每个副本稳定的网络名 `pg-0`、`pg-1` 和稳定的盘，启停有序），要么干脆**不自己跑**——用 RDS、Cloud SQL 这类托管服务，或者用成熟的 Operator，它会管备份、故障转移、扩容这些 Helm 一次性部署管不了的事。

**配置**。K8s 把配置抽成两类独立对象：**ConfigMap** 放不敏感的（日志级别、feature flag、DB 主机名），**Secret** 放敏感的（密码、token、证书）。两者都可以被多个 Pod 引用、单独修改、单独授权。注入方式两种：变成环境变量，或者挂成文件。

背后的原则还是刚才那一条，只是换了个对象：**配置也是一种不能待在镜像里的状态**。

判断标准很简单：**同一个镜像，在 dev 和 prod 跑出来的行为不一样的那部分，就是配置。** 它必须从外面注入，而不能编译进镜像——否则你就得为每个环境各构建一个镜像，而**"dev 测过的那个镜像，和 prod 上跑的那个镜像，是同一个"这个保证会当场消失**。这是 12-factor 里最有价值的一条。

顺带说说**为什么用环境变量、而不是配置文件**：环境变量是**进程启动时确定、之后不可变**的，这和不可变基础设施是一套的——配置要改就换 Pod，而不是原地热更新。挂成文件的方式适合大块配置（比如一整份 nginx.conf），代价是你得自己处理它会被 kubelet 悄悄更新这件事。

这里有一条必须说的：**Secret 并不加密**。

【敲】

```bash
kubectl get secret hello-db -o jsonpath='{.data.password}' | base64 -d; echo
```

【说】
明文就出来了，它只是 **base64 编码**。Secret 真正的价值在别处：RBAC 可以单独管控谁能读它；`kubectl get`、`describe`、`kubectl describe pod` 这些每天要敲的命令**都不会把它打印出来**——你贴给同事的日志、你的录屏里都不会带上它，而放在 ConfigMap 里的值会原样打印。真要管密钥，看 Sealed Secrets、External Secrets Operator 或者 Vault。

最后一个小坑，踩过的人会心一笑：**改了 ConfigMap 之后 `helm upgrade`，Pod 不会重启，新配置不生效。** 因为 Deployment 的 pod template 一个字节都没变——它引用的是 ConfigMap 的名字，名字没变，K8s 认为期望和现实一致，什么都不用做。标准解法是把 ConfigMap 的内容哈希写进 pod template 的 annotation：

```yaml
annotations:
  checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
```

内容一变、哈希就变、pod template 就变，于是正常触发一次滚动更新。

---

## 生命周期：滚动更新、探针与回滚

【说】
最后一节，也是我最想让你们看到的一节。只讲一件事：**版本怎么换**。

演示用同一份源码打了三个 tag：`v1` 和 `v2` 只有打印的版本号不同；`v3-broken` 是构建参数里塞了一个"启动即退出"，模拟一个**坏掉的版本**。

【敲】

```bash
helm upgrade hello ./05-lifecycle/chart
```

【互动 7】
**所有人把地址改回根路径 `/`，先敲一下**，确认现在拿到的是 `version v1`。

```bash
curl -s http://IP:8080/
```

接下来这一段，**我说"敲"你们就敲一下**，我会连着喊四五次。现在我发 v2：

```bash
helm upgrade hello ./05-lifecycle/chart --set image.tag=v2
```

**敲！**……**敲！**……**谁翻成 `version v2` 了，喊一声！**……**敲！**……**再敲！**
（喊到全场都是 v2 为止）中间这几轮里，有人拿到 v1、有人拿到 v2，**两个版本是同时在线的**。

**关键问题：这几轮里有人报错吗？有人拿到连接被拒绝、或者 502 吗？**（应该没有）

【说】
这不是运气好，是 chart 里写死的更新策略：

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```

- `maxUnavailable: 0` —— 任何时刻可用副本数都不许少于期望值。**新 Pod 没 Ready 之前，旧 Pod 一个都不许删。**
- `maxSurge: 1` —— 允许临时多出一个 Pod，给新版本腾地方。

所以那一小段 v1 和 v2 并存是必然的，**它就是不停机的代价**：你的应用必须能容忍新旧两个版本同时在线，这也是为什么数据库迁移要向后兼容。

现在**我故意发一个坏版本上去**。

【互动 8】

```bash
helm upgrade hello ./05-lifecycle/chart --set image.tag=v3-broken
```

【说】
（指分屏）我这边已经红了：`CrashLoopBackOff`，新 Pod 起不来。

**大家敲一下。再敲一下。谁看到 `v3` 了？谁报错了？**
（全场应该还是 `version v2`）

对，**我发了一个完全跑不起来的版本，线上还在正常服务 v2**，你们一个人都没受影响。这是今天最重要的一张图，两个机制在保护我：

- **readinessProbe 决定"能不能接流量"**。v3-broken 的进程启动即退出，从没监听过 8080，探针永远不成功，这个 Pod 永远不是 Ready，**Service 的名单里根本没有它**。
- **`maxUnavailable: 0` 决定"旧的能不能撤"**。滚动更新要等新 Pod Ready 才会删旧 Pod，而新 Pod 永远不 Ready，于是两个 v2 就一直留着。

探针不是可选项，这就是原因。

还有一件反直觉的事：

【敲】

```bash
helm history hello
```

【说】
看 revision 3 的状态：**`deployed`，Helm 认为自己成功了**。因为 `helm upgrade` 默认只负责把新 manifest 提交给 API Server，**不等 Pod 真的起来**。提交成功它就返回了，Pod 跑没跑起来是控制器之后的事。这个坑在 CI 里很致命：流水线绿了，线上是坏的。

先救火，回滚：

【敲】

```bash
helm rollback hello 2
kubectl get pods
helm history hello
```

【说】
一条命令，坏 Pod 没了。注意两个 v2 Pod 的 AGE——**它们从头到尾就没重启过**，这次回滚对它们来说只是"期望状态又变回了你"。

再看历史表：**回滚不是删除历史，而是追加一条新的 revision**。当前变成 revision 4，描述写着 `Rollback to 2`；revision 3 还在，随时可以再滚回去。

最后一条，这是**你们真正该抄走的写法**。刚才那套要人盯着：发完看 Pod、发现红了、手动回滚。CI 里没人盯，就该让 Helm 自己判断：

【敲】

```bash
helm upgrade hello ./05-lifecycle/chart --set image.tag=v3-broken \
  --atomic --wait --timeout 60s
```

【说】
（**这 60 秒什么都别讲**，让他们盯着分屏看新 Pod 起不来、Helm 卡在那儿等。这个"卡住"本身就是我要他们看的东西。等报错出来再开口。）

报错是：
`Error: UPGRADE FAILED: release hello failed, and has been rolled back due to atomic being set: context deadline exceeded`

三个参数在这一行报错里全能对上号：

- `--wait` —— 提交完不返回，一直等到所有 Pod 都 Ready。
- `--timeout 60s` —— 等多久算失败，也就是末尾那句 `context deadline exceeded`。**这个值要给够**，它包含拉镜像的时间，给太短会把一次正常但偏慢的发布判成失败。
- `--atomic` —— 一旦失败就自动回滚，也就是中间那句 `has been rolled back`。

一次失败的发布变成了一次"**什么都没发生**"：集群回到升级前的样子，CI 明确报错，**你们的屏幕全程是 v2**。生产里的 `helm upgrade` 基本都该是这个形状。

---

## 收尾

【说】
一小时过完，回头看这条线：

compose 只能在一台机器上把容器跑起来 → K8s 让你**声明期望**，控制器负责维持（副本、自愈、负载均衡）→ 手写 YAML 在多环境下会失控 → Helm 用 **Chart、Values、Release** 接手，还替你记账 → 于是**升级和回滚变成一条命令**，而探针和滚动更新策略保证了这个过程里你们一次请求都没丢。

【说】
再往后是三个方向，今天不展开，知道往哪儿看就行：

- **Kustomize**：做 patch 和 overlay，没有模板语言，kubectl 原生集成。和 Helm 不是二选一，可以叠着用。
- **GitOps（ArgoCD / Flux）**：注意它补的正是我们今天点出来的那个洞——**Helm 不 watch 集群**。GitOps 把一个控制器常驻在集群里，让"Git 里写的"变成期望状态、持续调谐，有人手改了自动改回去。**说到底，它就是把今天这个模式又套用了一层。**
- **Operator / CRD**：上面刚讲的那条路。

今天所有的代码、YAML、命令都在这个仓库里：`github.com/ShuaiweiYu/k8s-helm-tutorial`，每一节一个目录，clone 下来照着敲就能全部复现。我还把每一节写成了博客，链接一会儿发群里。

你们刚才一个小时里打的那些请求，最后都落在我这台笔记本上的一个 minikube 集群里——**这套东西在一台笔记本上就能跑起来**，这是我最想传达的一点。

谢谢，有什么问题？

【敲】（提问时间收尾）

```bash
helm uninstall hello
```