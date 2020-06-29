import argparse
import json
import os
import subprocess
import typing

file_dir = os.path.dirname(os.path.abspath(__file__))
nuclio_dir = os.path.join(file_dir,
                          os.path.pardir,  # hack
                          os.path.pardir)  # nuclio
function_examples_dir = os.path.join(nuclio_dir, "hack", "examples")
default_function_name = "empty"
default_runtime_to_handlers = [
    "golang@empty:Handler",
    "python:2.7@empty:handler",
    "python:3.6@empty:handler",
    "java@EmptyHandler",
    "dotnetcore@nuclio:empty",
    "nodejs@empty:handler",
]
all_runtimes = [
    default_runtime_to_handler.split("@")[0]
    for default_runtime_to_handler in default_runtime_to_handlers
]


# - compile list of runtimes
# - run nuctl deploy for each `runtime`
# - map deployed function host:port
# - run vegerat, save output in json
# - read output, generate details in table
# - save table to file


def _run_nuctl(nuctl_path, *cmd, check=True):
    return subprocess.run([nuctl_path, *cmd], check=check, stdout=subprocess.PIPE)


def _parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--nuctl-path",
                        help="Nuclio CLI ('nuctl') path",
                        required=True)
    parser.add_argument("--skip-deploy",
                        help="Whether to deploy functions first (Default: False)",
                        action="store_true")
    parser.add_argument("--function-http-max-workers",
                        help="Number of function http trigger workers. (Default: # CPUs",
                        default=os.cpu_count())
    parser.add_argument("--nuctl-platform",
                        help="Platform to deploy and benchmark on. (Default: local)",
                        default="local")
    parser.add_argument("--runtimes",
                        nargs="+",
                        help=f"Nuclio runtimes to benchmark (e.g.: python:3.6, Default: {all_runtimes})",
                        default=all_runtimes)
    parser.add_argument("--runtime-shared-function-name",
                        help=f"Function name to deploy to benchmark runtimes (Default: {default_function_name})",
                        default=default_function_name)
    parser.add_argument("--runtime-shared-function-path-dir",
                        help="A shared directory for benchmarked runtimes. "
                             "e.g.: /home/user/here is the runtime shared function path dir "
                             "and it contains subdirectories as follow "
                             "- /home/user/here/java/<function-name> "
                             "- /home/user/here/golang/<function-name> "
                             f"Default {function_examples_dir}",
                        default=function_examples_dir)
    parser.add_argument("--runtime-to-handlers",
                        nargs="+",
                        help="Map between each runtime to its function handler. "
                             "Separated with @ (e.g.: golang@main:Handler)",
                        default=default_runtime_to_handlers)
    return parser.parse_args()


def _resolve_fucntion_port(nuctl_path, nuctl_platform, function_name):
    get_function_response = _run_nuctl(nuctl_path,
                                       "--platform", nuctl_platform,
                                       "get", "function", function_name)

    # second line, at the second to last column
    # TODO: allow nuctl to extract function port more easily
    return get_function_response.stdout.decode().split("\n")[1].split("|")[-2].strip()


def run(nuctl_path: str,
        nuctl_platform: str,
        runtimes: typing.List[str],
        runtime_to_handlers: typing.List[str],
        http_max_workers: int,
        function_name: str,
        function_path_dir: str,
        skip_deploy: bool):
    # ["golang@empty:Handler"] -> {"golang": "empty:Handler"}
    runtime_to_handlers_as_dict = {
        runtime_to_handler.split("@")[0]: runtime_to_handler.split("@")[1]
        for runtime_to_handler in runtime_to_handlers
    }
    http_trigger = {"mh": {"kind": "http", "maxWorkers": http_max_workers}}
    for runtime in runtimes:
        print(f"Benchmarking runtime: {runtime}")
        runtime_as_function_name = runtime.replace(":", "-").replace(".", "")
        function_name = f"{runtime_as_function_name}-{function_name}"
        if not skip_deploy:
            _run_nuctl(nuctl_path,
                       "deploy",
                       function_name,
                       f"--verbose",
                       "--platform", nuctl_platform,
                       "--path", os.path.join(function_path_dir, runtime.split(":")[0], function_name),
                       "--runtime", runtime,
                       "--triggers", json.dumps(http_trigger),
                       "--handler", runtime_to_handlers_as_dict[runtime])

        function_port = _resolve_fucntion_port(nuctl_path, nuctl_platform, function_name)
        function_url = f"http://:{function_port}"
        vegeta_cmd = f"vegeta attack " \
                     f"-duration 3s " \
                     f"-rate 0 " \
                     f"-connections {http_max_workers} " \
                     f"-workers {http_max_workers} " \
                     f"-max-workers {http_max_workers}"
        # vegeta_report_cmd = f"vegeta report --type json"
        # vegeta_report_cmd = f"vegeta report"
        #
        # p = subprocess.run([
        #     "/bin/bash", "-c",
        #     f"echo 'POST {function_url}' "
        #     f"| {vegeta_cmd} > attack.bin"], stdout=subprocess.PIPE)
        # results = json.loads(p.stdout)
        # res = subprocess.run([
        #     "echo", f"'POST {function_url}' |",
        #     *vegeta_cmd.split(" ")
        # ], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True)
        # print(res)
        #
        # #  | {vegeta_cmd} | tee results.bin | vegeta report
        # # benchmark_results = subprocess.run([
        # #     "echo", "'POST {function_url}'",
        # #     "|", vegeta_cmd, "|", "tee", "results.bin"
        # # ], check=True, stdout=subprocess.PIPE)


if __name__ == '__main__':
    args = _parse_args()
    run(args.nuctl_path,
        args.nuctl_platform,
        args.runtimes,
        args.runtime_to_handlers,
        args.function_http_max_workers,
        args.runtime_shared_function_name,
        args.runtime_shared_function_path_dir,
        args.skip_deploy)
