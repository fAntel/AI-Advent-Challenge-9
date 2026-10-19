import kotlin.test.*
import kotlinx.serialization.json.*

class SchemaTest {
    private fun registry() { if (tools.isEmpty()) registerTools() }
    @Test fun exactSurfaceAndNoDestructiveFlags() {
        registry()
        assertEquals(25, tools.size)
        assertEquals(25, tools.map { it.name }.distinct().size)
        assertFalse(tools.any { it.name.contains("firmware") || it.name.contains("register") || it.name.contains("goal") })
        for (t in tools) assertFalse(t.input.getValue("properties").jsonObject.containsKey("resync"))
    }
    @Test fun rejectDeletionAndMalformedWeightsBeforeAuth() {
        registry()
        for (data in listOf(
            """{"weight":80,"measurementTime":1700000000000}""",
            """{"weight":-1,"measurementTime":1700000000}""",
            """{"weight":80,"measurementTime":1700000000,"isDeleted":true}""",
            """{"weight":80,"measurementTime":1700000000,"id":12}"""
        )) {
            val r = callTool(json.parseToJsonElement("""{"name":"add_weight","arguments":{"data":$data}}""").jsonObject)
            assertEquals(JsonPrimitive(true), r["isError"])
            assertTrue(r.toString().contains("Invalid input"))
        }
    }
    @Test fun updateNeedsIdAndBatchRejectsDuplicates() {
        registry()
        val w = """{"weight":80,"measurementTime":1700000000}"""
        for ((name, data) in listOf("update_weight" to w, "add_batch_weight" to "[$w,$w]")) {
            assertTrue(callTool(json.parseToJsonElement("""{"name":"$name","arguments":{"data":$data}}""").jsonObject).toString().contains("Invalid input"))
        }
    }
    @Test fun schemaHasNestedMuscleMassAndNullables() {
        registry()
        val t = tools.first { it.name == "add_weight" }
        val a = json.parseToJsonElement("""{"data":{"weight":80.2,"measurementTime":1700000000,"height":null,"additionalData":{"muscleMass":34.2}}}""")
        validate(a, t.input)
        t.prepare(a.jsonObject)
        assertTrue(t.input.toString().contains("muscleMass"))
    }
    @Test fun sleepEventsRejectWrongModuleBeforeAuth() {
        registry()
        val args = json.parseToJsonElement("""{"name":"upload_sleep_events","arguments":{"data":[{"module_id":0,"event_type":1,"rec_id":1,"timestamp":1700000000,"utc_offset":0,"sensor_id":"fixture"}]}}""").jsonObject
        assertTrue(callTool(args).toString().contains("Invalid input"))
    }
    @Test fun rejectsUnknownTopLevelAndWrongPrimitiveTypes() {
        registry()
        val t = tools.first { it.name == "download_weights_by_measurement_time" }
        for (raw in listOf("""{"fromTime":"1700000000"}""", """{"fromTime":1700000000,"resync":true}""")) {
            assertFailsWith<IllegalArgumentException> { validate(json.parseToJsonElement(raw), t.input) }
        }
    }
}
