<p align="center">
   <img width="360" src="https://raw.githubusercontent.com/hipoint-airpress/airpress/main/resources/admin/images/logo.svg" />
</p>

<p align="center"><b>AirPress </b> 是一个用Golang开发的轻量级CMS平台，高效快速.</p>

<p align="center"><a href="https://github.com/hipoint-airpress/airpress">hipoint-airpress/airpress</a> 的社区维护分支</p>

<p align="center">
<a href="https://github.com/hipoint-airpress/airpress/releases"><img alt="GitHub release" src="https://img.shields.io/github/release/hipoint-airpress/airpress.svg?style=flat-square&include_prereleases" /></a>
<a href="https://github.com/hipoint-airpress/airpress/releases"><img alt="GitHub All Releases" src="https://img.shields.io/github/downloads/hipoint-airpress/airpress/total.svg?style=flat-square" /></a>
<a href="https://hub.docker.com/r/h1dian/airpress"><img alt="Docker pulls" src="https://img.shields.io/docker/pulls/h1dian/airpress?style=flat-square" /></a>
<a href="https://github.com/hipoint-airpress/airpress/commits"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/hipoint-airpress/airpress.svg?style=flat-square" /></a>
</p>


## 📖 介绍

AirPress 致力于成为高效快速的开源CMS平台。

## 🔀 关于本项目

AirPress 克隆自 [go-sonic/sonic](https://github.com/go-sonic/sonic) 项目，重命名为 AirPress，并维护于 [hipoint-airpress/airpress](https://github.com/hipoint-airpress/airpress)。

上游项目自 2024 年 5 月起已无新的提交，因此 AirPress 现在是该项目持续维护的延续版本：

- **新功能**继续在本仓库开发。
- **Issue 在本仓库处理**：请到 [hipoint-airpress/airpress/issues](https://github.com/hipoint-airpress/airpress/issues) 反馈问题、提出需求，不要提到上游仓库。
- 已有的数据、配置文件和主题与上游保持兼容。

感谢 [Halo](https://github.com/halo-dev/) 项目组，本项目的灵感来自 Halo，前端项目 Fork 自 [Console](https://github.com/halo-dev/console)。

## 🚀 Features:
- 支持多种类型的数据库：SQLite、MySQL(TODO: PostgreSQL)
- 体积小: 安装包仅仅只有10Mb
- 高性能: 文章详情页可以达到2500 QPS(压测环境是: Intel Xeon Platinum 8260 4C 8G ,SQLite3)
- 支持更换主题
- 支持 Linux、Windows、Mac OS等主流操作系统，支持x86、x64、Arm、Arm64、MIPS等指令集架构
- 支持对象存储(MINIO、Google Cloud、AWS、AliYun)

## 🎊 Preview

![默认主题](https://raw.githubusercontent.com/hipoint-airpress/theme-anatole/main/screenshot.png)

![控制台](https://github.com/go-sonic/resources/raw/master/console-screenshot.png)

## 🧰 安装

**下载对应平台的安装包**
> 根据你的操作系统和指令集下载对应的安装包,注意要下载最新的版本
```bash
wget https://github.com/hipoint-airpress/airpress/releases/download/v1.0.3/airpress-linux-amd64.zip -O airpress.zip
```
**解压**
```bash
unzip -d airpress airpress.zip
```
**运行**
> 可以通过 -config选项来指定配置文件的位置
```bash
cd airpress
./airpress -config conf/config.yaml
```

**然后你就可以通过浏览器访问AirPress了，默认的端口是8080**

后台管理路径是 http://ip:port/admin

## 🔨️  构建
**1. 拉取项目**
```bash
git clone --recursive --depth 1 https://github.com/hipoint-airpress/airpress
```
**2. 运行**
```bash
cd airpress
go run main.go
```
> 如果你在windows上构建该项目，请确保你已经正确安装了gcc编译器，你可以在[这里](https://www.mingw-w64.org/getting-started/msys2/) 找到并安装mingw-w64 GNU工具链。

🚀 完成! 你的项目现在已经运行起来了。

## Docker
See: https://hub.docker.com/r/h1dian/airpress

## 主题生态

| Theme                                                                | 
|----------------------------------------------------------------------|
| [Anatole](https://github.com/hipoint-airpress/theme-anatole) |


## 如何贡献

非常欢迎你的加入！[提一个 Issue](https://github.com/hipoint-airpress/airpress/issues) 或者提交一个 Pull Request。


AirPress 遵循 [Contributor Covenant](http://contributor-covenant.org/version/1/3/0/) 行为规范。

### 贡献者

感谢以下参与项目的人：
<a href="https://github.com/hipoint-airpress/airpress/graphs/contributors"><img src="https://contrib.rocks/image?repo=hipoint-airpress/airpress" />></a>

## 📄 License

AirPress 的 Go 后端以及其余源码基于 [MIT License](/LICENSE.md) 开源。
