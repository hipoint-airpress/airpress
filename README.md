<p align="center">
   <img width="360" src="https://raw.githubusercontent.com/hipoint-airpress/airpress/main/resources/admin/images/logo.svg" />
</p>

<p align="center"><b>AirPress </b> is a Go Blogging Platform. Simple and Powerful.</p>

<p align="center">A community-maintained fork of <a href="https://github.com/hipoint-airpress/airpress">hipoint-airpress/airpress</a></p>

<p align="center">
<a href="https://github.com/hipoint-airpress/airpress/releases"><img alt="GitHub release" src="https://img.shields.io/github/release/hipoint-airpress/airpress.svg?style=flat-square&include_prereleases" /></a>
<a href="https://github.com/hipoint-airpress/airpress/releases"><img alt="GitHub All Releases" src="https://img.shields.io/github/downloads/hipoint-airpress/airpress/total.svg?style=flat-square" /></a>
<a href="https://hub.docker.com/r/h1dian/airpress"><img alt="Docker pulls" src="https://img.shields.io/docker/pulls/h1dian/airpress?style=flat-square" /></a>
<a href="https://github.com/hipoint-airpress/airpress/commits"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/hipoint-airpress/airpress.svg?style=flat-square" /></a>
</p>


English | [中文](doc/README_ZH.md)

## 📖 Introduction

AirPress is a high-performance blog system developed using golang.

## 🔀 About this fork

AirPress is a fork of the [go-sonic/sonic](https://github.com/go-sonic/sonic) project, renamed and maintained at [hipoint-airpress/airpress](https://github.com/hipoint-airpress/airpress).

Upstream has not received new commits since May 2024, so AirPress is now the actively maintained continuation of the project:

- **New features** keep being developed here.
- **Issues are triaged and fixed here** — please report bugs and request features in [hipoint-airpress/airpress/issues](https://github.com/hipoint-airpress/airpress/issues) instead of the upstream repository.
- Existing data, configuration files and themes remain compatible with the upstream project.

Thanks to the [Halo](https://github.com/halo-dev) project team, who inspired this project. The front-end is a project fork from Halo.

## 🚀 Features:
- Support multiple types of databases: SQLite、MySQL(TODO: PostgreSQL)
- Small: The installation file is only 10mb size
- High-performance: Post details page can withstand 2500 QPS(Enviroment:   Intel Xeon Platinum 8260 4C 8G ,SQLite3)
- Support changing theme
- Support Linux、Windows、Mac OS. And Support x86、x64、Arm、Arm64、MIPS
- Object storage(MINIO、Google Cloud、AWS、AliYun)


## 🎊 Preview

![Default Theme](https://raw.githubusercontent.com/hipoint-airpress/theme-anatole/main/screenshot.png)

![Console](https://github.com/go-sonic/resources/raw/master/console-screenshot.png)

## 🧰 Install

**Download the latest installation package**
> Please pay attention to the operating os and instruction set  and the version
```bash
wget https://github.com/hipoint-airpress/airpress/releases/latest/download/airpress-linux-amd64.zip -O airpress.zip
```
**Decompression**
```bash
unzip -d airpress airpress.zip
```
**Launch**
```bash
cd airpress
./airpress -config conf/config.yaml
```

**Initialization**


Open http://ip:port/admin#install

Next, you can access AirPress through the browser.

The URL of the admin console is http://ip:port/admin

The default port is 8080.

## 🔨️  Build
**1. Pull Project**
```bash
git clone --recursive --depth 1 https://github.com/hipoint-airpress/airpress
```
**2. Run**
```bash
cd airpress
go run main.go
```
> To compile this package on Windows, you must have the gcc compiler installed，for example the TDM-GCC Toolchain can be found ([here](https://jmeubank.github.io/tdm-gcc/)).

🚀 Done! Your project is now compiled and ready to use.

## Docker
See: https://hub.docker.com/r/h1dian/airpress

## Theme ecology

| Theme                                                                        | 
|------------------------------------------------------------------------------|
| [Anatole](https://github.com/hipoint-airpress/theme-anatole) |

## Contributing

Feel free to dive in! [Open an issue](https://github.com/hipoint-airpress/airpress/issues) or submit PRs.

AirPress follows the [Contributor Covenant](http://contributor-covenant.org/version/1/3/0/) Code of Conduct.

### Contributors

This project exists thanks to all the people who contribute. 
<a href="https://github.com/hipoint-airpress/airpress/graphs/contributors"><img src="https://contrib.rocks/image?repo=hipoint-airpress/airpress" /></a>

## 📄 License

The Go backend and the rest of the source code in `airpress` is available under the [MIT License](/LICENSE.md).


