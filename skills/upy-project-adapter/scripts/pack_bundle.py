#!/usr/bin/env python3
"""打包白名单 staging；manifest 使用 JSON（YAML 的子集），仅依赖标准库。"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import zipfile


def checked_path(stage, value):
    if not isinstance(value, str) or not value or '\\' in value:
        raise ValueError('产物路径必须是非空 POSIX 相对路径')
    path = PurePosixPath(value)
    if path.is_absolute() or '..' in path.parts:
        raise ValueError('产物路径不能越出 staging')
    return stage.joinpath(*path.parts)


def validate(stage):
    manifest = json.loads((stage / 'deploy.yaml').read_text())
    if not isinstance(manifest, dict):
        raise ValueError('manifest 必须是对象')
    for field in ('name', 'version'):
        value = manifest.get(field)
        if not isinstance(value, str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', value) or '..' in value:
            raise ValueError(f'{field} 必须是安全非空名称')
    kind = manifest.get('type')
    if kind not in ('static', 'docker', 'system'):
        raise ValueError('type 必须是 static/docker/system')
    for other in {'static', 'docker', 'system'} - {kind}:
        if other in manifest:
            raise ValueError('不能混用部署配置块')
    if kind == 'static':
        required = checked_path(stage, manifest.get('static', {}).get('directory', 'dist')) / 'index.html'
    elif kind == 'docker':
        docker = manifest.get('docker', {})
        if docker.get('mode') not in ('compose', 'container'):
            raise ValueError('docker.mode 必须是 compose/container')
        compose = docker['mode'] == 'compose'
        required = checked_path(stage, docker.get('composeFile' if compose else 'dockerfile', 'docker-compose.yml' if compose else 'Dockerfile'))
    else:
        target = manifest.get('target', {})
        if target.get('os') != 'linux' or target.get('arch') not in ('amd64', 'arm64'):
            raise ValueError('system 必须明确 Linux amd64/arm64 目标')
        command = manifest.get('system', {}).get('command')
        if not isinstance(command, list) or not command or not all(isinstance(x, str) and not any(c in x for c in '\x00\r\n') for x in command) or not command[0].strip():
            raise ValueError('system.command 必须是有效 argv 数组')
        if command[0].startswith('/'):
            # Java/Python runtime 由目标服务器验证，不能当作 bundle 内文件检查。
            required = None
        else:
            if not command[0].startswith('./'):
                raise ValueError('bundle 内 executable 使用 ./ 相对路径，服务器 runtime 使用绝对路径')
            wd = checked_path(stage, manifest['system'].get('workingDir', '.'))
            required = checked_path(wd, command[0])
            if not required.is_file() or not required.stat().st_mode & 0o111:
                raise ValueError('system executable 不存在或没有执行权限')
    if required is not None and not required.is_file():
        raise ValueError(f'缺少产物: {required.relative_to(stage)}')
    return manifest


def pack(stage, output):
    stage, output = stage.resolve(), output.resolve()
    if stage == output or stage in output.parents:
        raise ValueError('output 必须在 staging 外，避免将旧 ZIP 打入新包')
    # 先拒绝软链及非普通文件，确保校验不会读取 staging 外部内容。
    entries = sorted(stage.rglob('*'))
    for path in entries:
        if path.is_symlink() or not (path.is_file() or path.is_dir()):
            raise ValueError(f'不允许软链或特殊文件: {path}')
        if path.name in {'.env', 'service.env', '.git', 'id_rsa', 'id_ed25519'} or path.name.startswith('.env.') and path.name != '.env.example':
            raise ValueError(f'疑似敏感文件: {path}；请使用显式的非敏感配置文件')
    validate(stage)
    output.parent.mkdir(parents=True, exist_ok=True)
    if output.exists():
        raise ValueError('输出已存在，请选择新版本/新路径或显式删除旧产物')
    with zipfile.ZipFile(output, 'x', compression=zipfile.ZIP_DEFLATED) as archive:
        for path in entries:
            if path.is_file():
                info = zipfile.ZipInfo(path.relative_to(stage).as_posix(), date_time=(1980, 1, 1, 0, 0, 0))
                info.create_system = 3
                info.external_attr = (stat.S_IFREG | (0o755 if path.stat().st_mode & 0o111 else 0o644)) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                archive.writestr(info, path.read_bytes())
    digest = hashlib.sha256(output.read_bytes()).hexdigest()
    Path(str(output) + '.sha256').write_text(f'{digest}  {output.name}\n')
    return output


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--stage', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    try:
        print(pack(args.stage, args.output))
    except (ValueError, OSError, KeyError, TypeError, AttributeError) as error:
        parser.exit(1, f'打包失败: {error}\n')
