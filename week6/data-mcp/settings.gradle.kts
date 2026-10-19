pluginManagement { repositories { gradlePluginPortal(); mavenCentral() } }
rootProject.name = "healbe-data-mcp"
dependencyResolutionManagement {
    repositories {
        mavenCentral()
        maven {
            url = uri(providers.gradleProperty("nexusReleaseUrl").get())
            content { includeGroup("com.healbe") }
        }
        maven {
            url = uri(providers.gradleProperty("nexusPrivateUrl").get())
            credentials {
                username = providers.gradleProperty("nexusPrivateUsername").get()
                password = providers.gradleProperty("nexusPrivatePassword").get()
            }
            content { includeGroup("com.healbe") }
        }
    }
}
