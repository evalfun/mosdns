# mosdns

## 增加功能：自定义dns
通过使用api和数据库的方式动态设置和删除域名的解析记录。
### 支持特性：
- 支持a记录 aaaa记录 txt记录
- 支持SQLite和MySQL数据库
- 通过标准的的http api进行控制

### 配置方式：
```
plugins:
  - tag: "exec_cdns"    # 插件tag，可以自定义
    type: "custom_dns"  # 插件名称，必须是custom_dns
    args:
      # database_type: sqlite          # 数据库类型 只能填写 sqlite 或者 mysql
      # database_address: database.db  # 如果是sqlite数据库，填写数据库的文件路径
      database_type: mysql    
      database_address: mysql_user:mysql_password@tcp(127.0.0.1:3306)/mysql_database?charset=utf8mb4&parseTime=True&loc=Local
      # mysql 数据库填写数据库连接信息。

  - tag: main           # 最后将此插件注册到 main 执行队列中，就可以调用插件了。
    type: sequence
    args:
      - exec: query_summary query_start
      - exec: $exec_cdns

api:                    # 需要开启mosdns的api监听，否则插件的api将无法调用
  http: 0.0.0.0:8231
```

示例配置可以查看仓库里的config.yaml。此配置可以开箱即用。mosdns会先查询是否设置了自定义dns，如果没有查到就去查缓存。
未命中缓存时会判断是否是中国大陆域名，如果是中国大陆域名则转发到中国大陆的dns服务器。否则会转发到中国大陆外的服务器。
最后缓存结果以供下次查询。

### 域名匹配方式
有3种匹配方式：

1. 完整匹配

    写法：直接写整个域名。例如 www.baidu.com。只会完整匹配域名。

2. 通配符匹配

    写法：*.域名。例如 *.baidu.com。会匹配 www.baidu.com, tieba.baidu.com 但不会匹配 1.2.3.baidu.com, baidu.com

3. 域名匹配

    写法：domain:域名。例如 domain:baidu.com。会匹配 baidu.com和任意以.baidu.com结尾的域名，例如www.baidu.com，1.2.3.4.baidu.com

匹配优先级为完整匹配>通配符匹配>域名匹配。

### 记录类型
1. txt 

    文本记录，最大255字节

2. a 

    ipv4地址记录

3. aaaa 

    ipv6地址记录，兼容ipv4地址

### api调用方式：
在上面的配置中，插件tag为exec_cdns，那么api的url如下：
- POST /plugins/exec_cdns/delete

    删除记录。请求示例：
    ```
    {
        "Hostname": "要删除的域名匹配", 
        "Type": "a"|"aaaa"|"txt"
    }
    ```

- GET  /plugins/exec_cdns/list

    列出已经存在的记录。只会列出记录的域名匹配规则，不会列出对应的值。响应示例：
    ```
    {
        "RecordA": [
            {
                "Hostname": "*.t.flanc",
                "TTL": 200
            }
        ],
        "RecordAAAA": [
            {
                "Hostname": "domain:t.flanc",
                "TTL": 200
            }
        ],
        "RecordTXT": [
            {
                "Hostname": "t.flanc",
                "TTL": 200
            }
        ]
    }
    ```

- GET  /plugins/exec_cdns/query

    查找一个域名匹配规则对应的所有值。请求示例：

    GET /plugins/exec_cdns/query?hostname=域名匹配规则

    响应示例：

    ```
    {
        "RecordA": [
            "127.0.0.2"
        ],
        "RecordAAAA": [],
        "TXT": null
    }
    ```

    注意：当域名匹配规则对应值是null时代表系统中没有记录这个域名，会继续执行其他插件进行域名解析。

    如果对应的值是[]，那么会返回没有可以使用的记录。可以通过这个方式屏蔽某些域名的解析。

- POST /plugins/exec_cdns/set

    设置域名匹配规则和它对应的值。如果设置多个value，那么会以随机的顺序响应，以达到负载均衡的目的。请求示例：
    ```
    {
        "Hostname": "域名匹配规则",
        "Type": "a"|"aaaa"|"txt",
        "Value":[
            "127.0.0.2"
        ],
        "TTL": 200
    }
    ```




功能概述、配置方式、教程等，详见: [wiki](https://irine-sistiana.gitbook.io/mosdns-wiki/)

下载预编译文件、更新日志，详见: [release](https://github.com/IrineSistiana/mosdns/releases)

docker 镜像: [docker hub](https://hub.docker.com/r/irinesistiana/mosdns)
