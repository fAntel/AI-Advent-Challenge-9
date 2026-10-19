plugins {
    kotlin("jvm") version "2.1.0"
    kotlin("plugin.serialization") version "2.1.0"
    application
}
dependencies {
    implementation("com.healbe:server-api-jvm:1.6.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0")
    implementation("org.jetbrains.kotlinx:kotlinx-datetime:0.6.1")
    testImplementation(kotlin("test"))
}
kotlin { jvmToolchain(21) }
application { mainClass.set("MainKt") }
tasks.test { useJUnitPlatform() }
val referenceSources by configurations.creating { isTransitive = false }
dependencies { referenceSources("com.healbe:server-api-jvm:1.6.0:sources") }
tasks.register<Copy>("referenceSources") {
    from(referenceSources)
    into(layout.buildDirectory.dir("reference"))
}

// A self-contained application image for the build host; no Java install needed.
tasks.register<Exec>("packageApp") {
    dependsOn("installDist", "test")
    val destination = layout.buildDirectory.dir("package")
    doFirst { delete(destination) }
    commandLine("${System.getProperty("java.home")}/bin/jpackage",
        "--type", "app-image", "--name", "healbe-data-mcp",
        "--input", layout.buildDirectory.dir("install/healbe-data-mcp/lib").get().asFile.absolutePath,
        "--main-jar", "healbe-data-mcp.jar", "--main-class", "MainKt",
        "--dest", destination.get().asFile.absolutePath,
        "--java-options", "-Dfile.encoding=UTF-8")
}
