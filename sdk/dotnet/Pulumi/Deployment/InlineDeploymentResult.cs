﻿// Copyright 2016-2020, Pulumi Corporation

using System.Runtime.ExceptionServices;

namespace Pulumi
{
    internal class InlineDeploymentResult
    {
        public int ExitCode { get; set; }

        public ExceptionDispatchInfo? ExceptionDispatchInfo { get; set; }
    }
}
