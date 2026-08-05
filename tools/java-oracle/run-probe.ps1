param(
    [string]$EsperRoot = 'D:\Code\soc\esper',
    [string]$RepoRoot = '',
    [string]$Manifest = '',
    [string]$Output = '',
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
}
if ([string]::IsNullOrWhiteSpace($Manifest)) {
    $Manifest = Join-Path $RepoRoot 'compat\static-manifest.json'
}
if ([string]::IsNullOrWhiteSpace($Output)) {
    $Output = Join-Path $RepoRoot 'compat\java-execution-inventory.jsonl'
}

$javaHome = 'C:\Program Files\Microsoft\jdk-17.0.20.8-hotspot'
$mavenHome = 'C:\Users\baicai\AppData\Local\UniGetUI\Chocolatey\lib-bad\maven\3.9.16\apache-maven-3.9.16'
$java = Join-Path $javaHome 'bin\java.exe'
$javac = Join-Path $javaHome 'bin\javac.exe'
$mvn = Join-Path $mavenHome 'bin\mvn.cmd'

foreach ($path in @($java, $javac, $mvn)) {
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Required tool was not found: $path"
    }
}

$env:JAVA_HOME = $javaHome
$env:MAVEN_HOME = $mavenHome
$env:Path = "$javaHome\bin;$mavenHome\bin;$env:Path"

if (-not (Test-Path -LiteralPath $Manifest)) {
    throw "Manifest was not found: $Manifest"
}
if (-not (Test-Path -LiteralPath (Join-Path $EsperRoot 'regression-lib\target\classes'))) {
    $SkipBuild = $false
}

if (-not $SkipBuild) {
    & $mvn -f (Join-Path $EsperRoot 'pom.xml') -pl regression-run -am test-compile '-DskipTests=true' '-Dcheckstyle.skip=true' '-Dgpg.skip=true' '-Dfile.encoding=UTF-8' '-Dproject.build.sourceEncoding=UTF-8' '-Dproject.reporting.outputEncoding=UTF-8'
    if ($LASTEXITCODE -ne 0) {
        throw 'Maven test-compile failed'
    }
}

$manifestObject = Get-Content -Raw -LiteralPath $Manifest | ConvertFrom-Json
$cases = @($manifestObject.cases | Where-Object { $_.kind -eq 'regression-execution' })
if ($cases.Count -eq 0) {
    throw 'The manifest contains no regression-execution cases'
}

$outerCases = @(
    $cases |
        Group-Object { "$($_.package)|$($_.outerClass)" } |
        ForEach-Object { $_.Group | Select-Object -First 1 }
)

$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('esper-java-oracle-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempRoot | Out-Null
try {
    $input = Join-Path $tempRoot 'execution-cases.tsv'
    $probeClasses = Join-Path $tempRoot 'classes'
    $mavenClasspathFile = Join-Path $tempRoot 'maven-classpath.txt'
    New-Item -ItemType Directory -Path $probeClasses | Out-Null

    $lines = foreach ($case in $outerCases) {
        @($case.id, $case.outerClass, "$($case.package).$($case.outerClass)", $case.sourceFile) -join "`t"
    }
    Set-Content -LiteralPath $input -Value $lines -Encoding utf8

    & $javac '-encoding' 'UTF-8' '-d' $probeClasses (Join-Path $PSScriptRoot 'ExecutionInventory.java')
    if ($LASTEXITCODE -ne 0) {
        throw 'javac failed for ExecutionInventory.java'
    }

    & $mvn -f (Join-Path $EsperRoot 'pom.xml') -pl regression-run dependency:build-classpath "-Dmdep.outputFile=$mavenClasspathFile" '-Dmdep.includeScope=test' '-Dcheckstyle.skip=true' '-DskipTests=true' '-Dgpg.skip=true' '-Dfile.encoding=UTF-8' '-Dproject.build.sourceEncoding=UTF-8' '-Dproject.reporting.outputEncoding=UTF-8'
    if ($LASTEXITCODE -ne 0) {
        throw 'Maven dependency:build-classpath failed'
    }

    $classpathEntries = @(
        (Join-Path $EsperRoot 'common\target\classes'),
        (Join-Path $EsperRoot 'common-avro\target\classes'),
        (Join-Path $EsperRoot 'common-xmlxsd\target\classes'),
        (Join-Path $EsperRoot 'compiler\target\classes'),
        (Join-Path $EsperRoot 'runtime\target\classes'),
        (Join-Path $EsperRoot 'regression-lib\target\classes'),
        (Join-Path $EsperRoot 'regression-run\target\test-classes'),
        (Join-Path $EsperRoot 'regression-run\etc')
    )
    $classpathEntries += (Get-Content -Raw -LiteralPath $mavenClasspathFile).Trim().Split(';')
    $classpath = ($classpathEntries | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) -join ';'

    $parent = Split-Path -Parent $Output
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    & $java '-cp' "$probeClasses;$classpath" 'ExecutionInventory' $input $Output
    if ($LASTEXITCODE -ne 0) {
        throw 'Java execution inventory probe failed'
    }

    $inventory = @(Get-Content -LiteralPath $Output | ForEach-Object { $_ | ConvertFrom-Json })
    $ok = @($inventory | Where-Object status -eq 'ok').Count
    $ignored = @($inventory | Where-Object status -eq 'ignored').Count
    $errors = @($inventory | Where-Object status -eq 'error').Count
    "staticCases=$($cases.Count) outerCases=$($outerCases.Count) records=$($inventory.Count) ok=$ok ignored=$ignored errors=$errors output=$Output"
} finally {
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}
