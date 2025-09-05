# Turnout CLI: Universal Deployment Schema Translator

## Executive Summary

Turnout is a command-line tool that parses existing deployment configurations from multiple sources and translates them into a unified schema, enabling deployment to platforms that don't natively support those configuration formats. The primary use case is enabling Railway users to deploy complex applications currently defined in Kubernetes manifests, Helm charts, Docker Compose files, or similar formats without manual translation.

Think of it like a "universal deployment AST" where the pipeline is: parse -> normalize -> validate/enrich -> export (e.g. to Railway)

## Problem Statement

Organizations have significant investments in deployment configurations written for specific orchestration platforms. When migrating to or evaluating Railway, they face the manual effort of translating these configurations, which creates friction in adoption. Additionally, maintaining parallel configuration formats for different platforms leads to configuration drift and increased operational overhead.

## Goals and Success Criteria

The primary goal is to automatically extract deployment semantics from existing configuration formats and generate Railway-compatible deployment specifications. Success will be measured by the tool's ability to handle common deployment patterns without manual intervention for at least 80% of standard web application architectures.

Key success metrics include accurate extraction of service definitions, environment variable mappings with proper sensitivity classification, service dependency graphs, and health check configurations. The tool should produce deterministic output given the same input and provide clear error messages when encountering unsupported patterns.

## Functional Requirements

### Input Format Support

The tool must support parsing and semantic extraction from Docker Compose files (versions 2 and 3), Dockerfiles with their build arguments and multi-stage builds, Kubernetes manifests including Deployments, Services, ConfigMaps, and Secrets, Helm charts with value overrides, environment files in dotenv format, and SOPS-encrypted configuration files.

### Core Extraction Capabilities

The system must identify all deployable services within the provided configurations, determining for each service its container image or build context, start command and arguments, exposed ports and network configuration, and resource requirements including CPU and memory limits.

For environment configuration, the tool must extract all environment variables, classify them by sensitivity level (detecting patterns for secrets, API keys, and database credentials), resolve variable references between services, and maintain the precedence order when variables are defined in multiple locations.

Service relationships require parsing dependency declarations, identifying shared volumes and network configurations, detecting database connections and service meshes, and preserving initialization order requirements.

### Output Schema

The unified schema must represent services as first-class entities with their runtime requirements, environment variables with type hints and sensitivity markers, inter-service dependencies and communication patterns, health check definitions including paths and expected responses, and deployment constraints such as region requirements or compliance needs.

## Technical Approach

The architecture should follow a pipeline model with distinct phases. The parsing phase uses format-specific parsers to build initial abstract syntax trees. The normalization phase transforms these ASTs into a common intermediate representation, resolving format-specific idioms into universal concepts. The enrichment phase adds semantic information through pattern matching and type inference. The validation phase ensures the configuration is internally consistent and deployable. Finally, the export phase generates platform-specific configuration from the intermediate representation.

The intermediate representation should capture the essential deployment semantics while abstracting away platform-specific implementation details. This includes service definitions with their runtime characteristics, a typed environment variable model with reference resolution, a dependency graph with initialization ordering, and resource specifications normalized across different unit systems.

## Constraints and Limitations

The tool will not attempt to translate platform-specific features that have no equivalent in the target platform, such as Kubernetes operators, custom resource definitions, or complex networking policies. Stateful workloads requiring specific storage classes or persistent volume claims will require manual review. Advanced orchestration features like blue-green deployments or canary releases will not be automatically configured.

The tool should clearly communicate when it encounters patterns it cannot translate, providing specific guidance on manual steps required. It should never silently drop configuration that might be critical to application function.

## Implementation Priorities

Phase one focuses on Docker Compose support as the initial input format, establishing the pipeline architecture and intermediate representation. This phase includes basic service extraction, environment variable handling, and Railway export functionality.

Phase two adds Dockerfile parsing to extract build-time configuration and Kubernetes manifest support for common resource types. This phase also introduces the type inference system for environment variables.

Phase three extends support to Helm charts with template resolution and SOPS integration for encrypted secrets. This phase also adds validation and warning systems for configuration incompatibilities.

## Risk Mitigation

The primary technical risk is the semantic gap between different orchestration platforms. This will be addressed by focusing on common patterns first and providing clear documentation of supported features. The tool should be conservative in its translations, preferring to fail explicitly rather than produce incorrect configurations.

Data security risks around sensitive configuration data will be mitigated by never logging or caching decrypted secrets, providing audit trails for secret access, and supporting standard secret management integrations.

## Validation Strategy

The tool's accuracy will be validated through a test suite of real-world applications spanning different architectural patterns. Each supported input format will have comprehensive parsing tests, and the export functionality will be validated by deploying the generated configurations to ensure they function correctly.
