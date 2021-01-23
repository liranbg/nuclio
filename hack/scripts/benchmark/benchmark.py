# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import json
import os
import pathlib
import shlex
import subprocess

"""
benchmark.py provides a simple yet quick way to run  HTTP load benchmarking tests against Nuclio function runtimes,
 such as (go, python, ...etc).

Benchmarking output contains:
 - HTML file containing a graph comparing each runtime latency (ms) over time
 - Binary files containing the requests were made during benchmarking
 - Descriptive information about total requests, throughput, duration, etc.

Prerequisites:
 - vegeta                  - https://github.com/tsenart/vegeta
 - python 3.7 and above    - https://realpython.com/intro-to-pyenv (pyenv install 3.7)       

Usage: `python benchmark.py --help`
"""


class Constants(object):
    project_dirpath = pathlib.Path(__file__).absolute().parent.parent.parent.parent
    function_examples_dirpath = pathlib.Path(project_dirpath, "hack", "examples")
    default_nuctl_path = pathlib.Path(project_dirpath, "nuctl")
    default_workdir = pathlib.Path(project_dirpath, ".benchmarking")
    plot_filepath = pathlib.Path(default_workdir, "plot.html")
    cpu_count = os.cpu_count()


class Runtimes(object):
    golang = "golang"
    python36 = "python:3.6"
    java = "java"
    nodejs = "nodejs"
    dotnetcore = "dotnetcore"

    # TODO: support benchmarking - add "empty function'
    # shell = "shell"
    # ruby = "ruby"

    @staticmethod
    def runtime_to_bm_function_handler(runtime):
        return {
            Runtimes.golang: "empty:Handler",
            Runtimes.python36: "empty:handler",
            Runtimes.java: "EmptyHandler",
            Runtimes.nodejs: "empty:handler",
            Runtimes.dotnetcore: "nuclio:empty",
        }[runtime]

    @staticmethod
    def all():
        return [
            Runtimes.golang,
            Runtimes.python36,
            Runtimes.java,
            Runtimes.nodejs,
            Runtimes.dotnetcore,
        ]


class VegetaClient(object):
    def __init__(self, workdir):
        self._workdir = workdir

    def attack(self, function_name, function_url, concurrent_requests):
        vegeta_cmd = "vegeta attack" \
                     f" -name {function_name}" \
                     " -duration 10s" \
                     " -rate 0" \
                     f" -connections {concurrent_requests}" \
                     f" -workers {concurrent_requests}" \
                     f" -max-workers {concurrent_requests}" \
                     f" -output {function_name}.bin"

        self._log(f"Attacking command - {vegeta_cmd}")
        subprocess.run(shlex.split(vegeta_cmd),
                       cwd=self._workdir,
                       check=True,
                       stdout=subprocess.PIPE,
                       input=f"GET {function_url}".encode(),
                       timeout=30)

    def plot(self, bin_names, output_filepath):
        encoded_bin_names = " ".join(f"{bin_name}.bin" for bin_name in bin_names)
        plot_cmd = f"vegeta plot --title 'Nuclio functions benchmarking' {encoded_bin_names}"
        self._log(f"Plotting command - {plot_cmd}")
        with open(output_filepath, "w") as outfile:
            subprocess.run(shlex.split(plot_cmd), cwd=self._workdir, check=True, stdout=outfile)

    def report(self, bin_name):
        report_cmd = f"vegeta report {bin_name}.bin"
        self._log(f"Reporting command - {report_cmd}")
        subprocess.run(shlex.split(report_cmd), cwd=self._workdir, check=True)

    def _log(self, content):
        print(f"\t[vegeta]: {content}")


def run(args):
    http_trigger = {"benchmark": {"kind": "http", "maxWorkers": args.function_http_max_workers}}
    vegeta_client = VegetaClient(args.workdir)
    function_names = []
    runtimes = _resolve_runtimes(args.runtimes)
    print(f"Benchmarking runtimes {runtimes}")
    for runtime in runtimes:
        os.makedirs(args.workdir, exist_ok=True)

        function_name = _resolve_function_name(runtime)
        if not args.skip_deploy:
            print(f"\t[{runtime}]: Deploying function {function_name}")
            function_path = pathlib.Path(Constants.function_examples_dirpath, runtime.split(":")[0], "empty")
            deploy_cmd = _resolve_deploy_cmd(function_name,
                                             args.verbose_deploy,
                                             runtime,
                                             args.nuctl_platform,
                                             function_path,
                                             http_trigger)
            _run_nuctl(args.nuctl_path, deploy_cmd)

        function_port = _get_function_port(args.nuctl_path, args.nuctl_platform, function_name)
        function_url = f"http://{args.function_url}:{function_port}"

        print(f"\t[{runtime}]: Benchmarking function {function_name} @ {function_url}")
        vegeta_client.attack(function_name, function_url, args.function_http_max_workers)
        vegeta_client.report(function_name)
        function_names.append(function_name)

    print(f">> Plotting {function_names} to {Constants.plot_filepath}")
    vegeta_client.plot(function_names, Constants.plot_filepath)
    print(f"Finished benchmarking.")


def _get_function_port(nuctl_path, nuctl_platform, function_name):
    cmd = f"get function --platform {nuctl_platform} --output json {function_name}"
    get_function_response = _run_nuctl(nuctl_path, cmd, capture_output=True)
    return json.loads(get_function_response.stdout.decode())["status"]["httpPort"]


def _resolve_deploy_cmd(function_name, verbose_deploy, runtime, nuctl_platform, function_path, http_trigger):
    deploy_cmd = f"deploy {function_name}"
    if verbose_deploy:
        deploy_cmd += " --verbose"
    deploy_cmd += f" --runtime {runtime}"
    deploy_cmd += f" --platform {nuctl_platform}"
    deploy_cmd += f" --path {function_path}"
    deploy_cmd += f" --triggers {shlex.quote(json.dumps(http_trigger))}"
    deploy_cmd += f" --handler {Runtimes.runtime_to_bm_function_handler(runtime)}"
    deploy_cmd += " --http-trigger-service-type nodePort"
    deploy_cmd += " --no-pull"
    return deploy_cmd


def _resolve_function_name(runtime):
    formatted_runtime = runtime.replace(":", "-").replace(".", "")
    return f"{formatted_runtime}-bm"


def _run_nuctl(nuctl_path, cmd, check=True, capture_output=False):
    print(f"\t[nuctl]: running command {cmd}")
    return subprocess.run(shlex.split(f"{nuctl_path} {cmd}"), check=check, capture_output=capture_output)


def _resolve_runtimes(runtimes):
    encoded_runtimes = runtimes
    if runtimes == "all":
        encoded_runtimes = ",".join(Runtimes.all())
    return encoded_runtimes.split(",")


def _parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--nuctl-path",
                        help=f"Nuclio CLI ('nuctl') path (Default: {Constants.default_nuctl_path}",
                        default=Constants.default_nuctl_path)
    parser.add_argument("--verbose-deploy",
                        help="Use verbose flag when deploying with nuctl",
                        action="store_true")
    parser.add_argument("--skip-deploy",
                        help="Whether to deploy functions first (Default: False)",
                        action="store_true")
    parser.add_argument("--function-url",
                        help="Function url to use for HTTP requests (Default: localhost)",
                        default="localhost")
    parser.add_argument("--function-http-max-workers",
                        help=f"Number of function http trigger workers. (Default: # CPUs - {Constants.cpu_count})",
                        default=Constants.cpu_count)
    parser.add_argument("--nuctl-platform",
                        help="Platform to deploy and benchmark on. (Default: auto)",
                        choices=["local", "kube", "auth"],
                        default="auto")
    parser.add_argument("--runtimes",
                        help=f"A comma delimited (,) list of Nuclio runtimes to benchmark (Default: all)",
                        default="all")
    parser.add_argument("--workdir",
                        help=f"Workdir to store benchmarking artifacts (Default: {Constants.default_workdir}",
                        default=Constants.default_workdir)
    return parser.parse_args()


if __name__ == '__main__':
    parsed_args = _parse_args()
    run(parsed_args)
