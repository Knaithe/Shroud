import paramiko
import sys
import time

def run_on_host(host, port, user, password, commands, timeout=60):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, port=port, username=user, password=password, timeout=15)
    results = []
    for cmd in commands:
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout)
        out = stdout.read().decode('utf-8', errors='replace')
        err = stderr.read().decode('utf-8', errors='replace')
        code = stdout.channel.recv_exit_status()
        results.append((cmd, code, out, err))
        print(f"[{code}] {cmd}")
        if out.strip():
            print(out.strip())
        if err.strip():
            print(f"STDERR: {err.strip()}")
        print("---")
    client.close()
    return results

def run_jump(jump_host, jump_port, jump_user, jump_pass,
             target_host, target_port, target_user, target_pass,
             commands, timeout=60):
    jump = paramiko.SSHClient()
    jump.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    jump.connect(jump_host, port=jump_port, username=jump_user, password=jump_pass, timeout=15)
    transport = jump.get_transport()
    channel = transport.open_channel('direct-tcpip', (target_host, target_port), ('127.0.0.1', 0))
    target = paramiko.SSHClient()
    target.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    target.connect(target_host, port=target_port, username=target_user, password=target_pass,
                   sock=channel, timeout=15)
    results = []
    for cmd in commands:
        stdin, stdout, stderr = target.exec_command(cmd, timeout=timeout)
        out = stdout.read().decode('utf-8', errors='replace')
        err = stderr.read().decode('utf-8', errors='replace')
        code = stdout.channel.recv_exit_status()
        results.append((cmd, code, out, err))
        print(f"[{code}] {cmd}")
        if out.strip():
            print(out.strip())
        if err.strip():
            print(f"STDERR: {err.strip()}")
        print("---")
    target.close()
    jump.close()
    return results

def scp_to_host(host, port, user, password, local_path, remote_path):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, port=port, username=user, password=password, timeout=15)
    sftp = client.open_sftp()
    sftp.put(local_path, remote_path)
    sftp.close()
    client.close()

def scp_via_jump(jump_host, jump_port, jump_user, jump_pass,
                 target_host, target_port, target_user, target_pass,
                 local_path, remote_path):
    jump = paramiko.SSHClient()
    jump.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    jump.connect(jump_host, port=jump_port, username=jump_user, password=jump_pass, timeout=15)
    transport = jump.get_transport()
    channel = transport.open_channel('direct-tcpip', (target_host, target_port), ('127.0.0.1', 0))
    target = paramiko.SSHClient()
    target.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    target.connect(target_host, port=target_port, username=target_user, password=target_pass,
                   sock=channel, timeout=15)
    sftp = target.open_sftp()
    sftp.put(local_path, remote_path)
    sftp.close()
    target.close()
    jump.close()

def scp_from_host(host, port, user, password, remote_path, local_path):
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, port=port, username=user, password=password, timeout=15)
    sftp = client.open_sftp()
    sftp.get(remote_path, local_path)
    sftp.close()
    client.close()

if __name__ == '__main__':
    print("ssh_helper loaded")
