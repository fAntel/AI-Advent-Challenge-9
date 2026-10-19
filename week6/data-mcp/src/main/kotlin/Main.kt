@file:OptIn(kotlinx.serialization.ExperimentalSerializationApi::class)

import com.healbe.healbesdk.logger.Logger
import com.healbe.healbesdk.models.healthdata.*
import com.healbe.healbesdk.serverApi.*
import com.healbe.healbesdk.serverApi.healthData.*
import com.healbe.healbesdk.serverApi.healthData.entity.*
import kotlinx.coroutines.runBlocking
import kotlinx.datetime.Instant
import kotlinx.serialization.*
import kotlinx.serialization.builtins.*
import kotlinx.serialization.descriptors.*
import kotlinx.serialization.json.*

val json = Json { encodeDefaults = true; explicitNulls = false }
fun obj(vararg pairs: Pair<String, JsonElement>) = JsonObject(mapOf(*pairs))
fun str(s: String) = JsonPrimitive(s)
fun number(n: Number) = JsonPrimitive(n)
fun JsonObject.text(key: String) = this[key]?.jsonPrimitive?.contentOrNull
fun JsonObject.long(key: String) = this[key]?.jsonPrimitive?.longOrNull
fun objectSchema(properties: Map<String, JsonElement>, required: List<String> = emptyList()) = obj(
    "type" to str("object"), "properties" to JsonObject(properties),
    "required" to JsonArray(required.map(::str)), "additionalProperties" to JsonPrimitive(false))
fun scalar(type: String, description: String) = obj("type" to str(type), "description" to str(description))

fun schema(d: SerialDescriptor): JsonObject {
    val base = when (d.kind) {
        PrimitiveKind.STRING, PrimitiveKind.CHAR -> scalar("string", "")
        PrimitiveKind.BOOLEAN -> scalar("boolean", "")
        PrimitiveKind.BYTE, PrimitiveKind.SHORT, PrimitiveKind.INT, PrimitiveKind.LONG -> scalar("integer", "")
        PrimitiveKind.FLOAT, PrimitiveKind.DOUBLE -> scalar("number", "")
        SerialKind.ENUM -> obj("type" to str("string"), "enum" to JsonArray((0 until d.elementsCount).map { str(d.getElementName(it)) }))
        StructureKind.LIST -> obj("type" to str("array"), "items" to schema(d.getElementDescriptor(0)))
        StructureKind.MAP -> obj("type" to str("object"), "additionalProperties" to schema(d.getElementDescriptor(1)))
        else -> objectSchema((0 until d.elementsCount).associate { i ->
            val name = d.getElementName(i)
            val child = schema(d.getElementDescriptor(i)).toMutableMap()
            val description = when {
                name == "measurementTime" || name == "timestamp" -> "Unix timestamp in UTC SECONDS, not milliseconds."
                name == "utc_offset" -> "UTC offset in seconds."
                name == "weight" || name == "muscleMass" -> "Mass in kilograms."
                name == "height" -> "Height in centimeters."
                name == "sensor_id" -> "GoBe sensor identifier; use an existing account sensor when testing wearable display."
                name == "event_data" -> "Optional Base64-encoded event payload; caller must encode it."
                name == "module_id" -> "Event module: sleep=1024, system=0; other IDs require domain knowledge."
                name == "event_type" -> "Bitmask of event flags; use known event definitions."
                name == "recordIndex" || name == "rec_id" -> "Record index; together with timestamp and sensor identifies an existing record that may be overwritten."
                else -> ""
            }
            if (description.isNotEmpty()) child["description"] = str(description)
            name to JsonObject(child)
        }, (0 until d.elementsCount).filter { !d.isElementOptional(it) }.map { d.getElementName(it) })
    }
    return if (d.isNullable) obj("anyOf" to JsonArray(listOf(base, obj("type" to str("null"))))) else base
}

// Validate the advertised subset of JSON Schema before authentication or any mutations.
fun validate(value: JsonElement, s: JsonObject, path: String = "arguments") {
    s["anyOf"]?.let { alternatives ->
        if (alternatives.jsonArray.none { runCatching { validate(value, it.jsonObject, path) }.isSuccess })
            throw IllegalArgumentException("$path has an invalid type")
        return
    }
    when (s.text("type")) {
        "object" -> {
            require(value is JsonObject) { "$path must be an object" }
            val props = s["properties"]?.jsonObject ?: JsonObject(emptyMap())
            for (key in s["required"]?.jsonArray.orEmpty()) require(key.jsonPrimitive.content in value) { "$path missing ${key.jsonPrimitive.content}" }
            for ((key, v) in value) {
                if (key in props) validate(v, props.getValue(key).jsonObject, "$path.$key")
                else if (s["additionalProperties"] == JsonPrimitive(false)) throw IllegalArgumentException("$path has unknown field")
                else (s["additionalProperties"] as? JsonObject)?.let { validate(v, it, "$path.*") }
            }
        }
        "array" -> {
            require(value is JsonArray) { "$path must be an array" }
            require(value.size >= (s.long("minItems") ?: 0) && value.size <= (s.long("maxItems") ?: 10000)) { "$path has invalid size" }
            value.forEachIndexed { i, v -> validate(v, s.getValue("items").jsonObject, "$path[$i]") }
        }
        "string" -> require(value is JsonPrimitive && value.isString) { "$path must be a string" }
        "integer" -> require(value is JsonPrimitive && !value.isString && value.longOrNull != null) { "$path must be an integer" }
        "number" -> require(value is JsonPrimitive && !value.isString && value.doubleOrNull?.isFinite() == true) { "$path must be a finite number" }
        "boolean" -> require(value is JsonPrimitive && !value.isString && value.booleanOrNull != null) { "$path must be boolean" }
        "null" -> require(value == JsonNull) { "$path must be null" }
    }
    s["enum"]?.let { require(value in it.jsonArray) { "$path has an invalid enum value" } }
}

@Serializable
data class WeightInput(
    override val weight: Double,
    override val measurementTime: Long,
    override val source: Source = Source.HEALBE,
    override val id: Long? = null,
    override val height: Double? = null,
    override val bmi: Double? = null,
    override val bodyFatRate: Double? = null,
    override val additionalData: Composition? = null,
    override val deviceId: String = "",
    override val outerId: String = ""
) : HDWeight {
    @Transient override val isDeleted: Boolean = false
    @Transient override val lastUpdate: Long? = null
    fun check(update: Boolean) {
        require(weight > 0 && weight <= 1000) { "weight must be in (0,1000] kg" }
        require(measurementTime in 1..4102444800L) { "measurementTime must be Unix seconds before 2100" }
        require(if (update) id != null && id > 0 else id == null) { "id required for update and forbidden for add" }
        require(height == null || height > 0 && height <= 300) { "height must be centimeters" }
        require(bodyFatRate == null || bodyFatRate in 0.0..100.0) { "bodyFatRate must be percent" }
    }
}

@Serializable
data class Composition(
    override val subcutaneousFatRate: Double? = null,
    override val visceralFatIndex: Int? = null,
    override val rateOfMuscle: Double? = null,
    override val basalMetabolicRate: Double? = null,
    override val physicalBodyAge: Int? = null,
    override val boneMass: Double? = null,
    override val waterContent: Double? = null,
    override val proteinPercentage: Double? = null,
    override val standardWeight: Double? = null,
    override val controlWeight: Double? = null,
    override val fatMass: Double? = null,
    override val weightWithoutFat: Double? = null,
    override val muscleMass: Double? = null,
    override val proteinMass: Double? = null,
    override val fatLevel: FatLevel? = null
) : HDAdditionalWeightData
fun weightOutput(w: HDWeight): JsonElement = json.encodeToJsonElement(WeightInput(w.weight, w.measurementTime, w.source, w.id, w.height, w.bmi, w.bodyFatRate, w.additionalData?.let { d -> Composition(d.subcutaneousFatRate, d.visceralFatIndex, d.rateOfMuscle, d.basalMetabolicRate, d.physicalBodyAge, d.boneMass, d.waterContent, d.proteinPercentage, d.standardWeight, d.controlWeight, d.fatMass, d.weightWithoutFat, d.muscleMass, d.proteinMass, d.fatLevel) }, w.deviceId, w.outerId))

data class Tool(val name: String, val description: String, val input: JsonObject, val readOnly: Boolean,
                val prepare: (JsonObject) -> (suspend (ServerClient) -> JsonElement)) {
    fun definition() = obj("name" to str(name), "description" to str(description), "inputSchema" to input,
        "annotations" to obj("readOnlyHint" to JsonPrimitive(readOnly), "destructiveHint" to JsonPrimitive(!readOnly),
            "idempotentHint" to JsonPrimitive(readOnly), "openWorldHint" to JsonPrimitive(true)))
}
val authProperties = mapOf(
    "login" to scalar("string", "Existing account email. Omit both login/password to use the privately configured demo account."),
    "password" to scalar("string", "Account password; sent to the harness/model if supplied. Never repeat it in the answer."))
fun inputs(props: Map<String, JsonElement>, required: List<String> = emptyList()) = objectSchema(authProperties + props, required)
fun arraySchema(item: JsonElement) = obj("type" to str("array"), "items" to item, "minItems" to number(1), "maxItems" to number(100))
val tools = mutableListOf<Tool>()

fun <T : NetworkHealthDataRecord> dataTools(
    suffix: String, serializer: KSerializer<T>,
    download: suspend (DataService, JsonObject) -> DownloadResponse<out NetworkHealthDataRecord>,
    encodeDownload: (DownloadResponse<out NetworkHealthDataRecord>) -> JsonElement,
    upload: suspend (DataService, List<T>) -> StatusResponse,
    full: Boolean = false, events: Boolean = false
) {
    val filters = mutableMapOf<String, JsonElement>(
        "fromTime" to scalar("integer", "Inclusive Unix timestamp in UTC seconds."),
        "toTime" to scalar("integer", "End Unix timestamp in UTC seconds."),
        "limit" to scalar("integer", "Page size, 1..100; defaults to 50. Inspect count/total/groups; never assume a page is the whole dataset."),
        "sensorId" to scalar("string", "Optional GoBe identifier."))
    if (!full) filters["from"] = scalar("integer", "Starting record index, not a timestamp.")
    if (events) filters["moduleId"] = scalar("integer", "Optional event module ID.")
    tools += Tool("download_$suffix", "Download $suffix through DataService. Times are seconds. Authenticates internally. Returns one page with library count/total metadata.", inputs(filters), true) { a ->
        require((a.long("limit") ?: 50) in 1..100) { "limit must be 1..100" }
        require(a.long("fromTime") == null || a.long("toTime") == null || a.long("fromTime")!! <= a.long("toTime")!!) { "fromTime must not exceed toTime" }
        val action: suspend (ServerClient) -> JsonElement = { c ->
            val response = download(c.getDataService(), a)
            require(response.responseCode?.let { it.isNotEmpty() && it.all { code -> code == 0 } } ?: (response.dataGroups != null)) { "server rejected download" }
            encodeDownload(response)
        }; action
    }
    tools += Tool("upload_$suffix", "Upload 1..100 synthetic $suffix records. Matching record identities may be overwritten. resync is always false. Use the exact schema and coherent timestamps/sensor IDs. Read back to verify; never blindly repeat uncertain uploads.", inputs(mapOf("data" to arraySchema(schema(serializer.descriptor))), listOf("data")), false) { a ->
        val records = json.decodeFromJsonElement(ListSerializer(serializer), a.getValue("data"))
        if (suffix == "sleep_events" || suffix == "system_events") {
            val expected = if (suffix == "sleep_events") EventNetwork.SLEEP_MODULE_ID else EventNetwork.SYSTEM_MODULE_ID
            require(records.all { (it as EventNetwork).moduleId == expected }) { "module_id must be $expected for $suffix" }
        }
        require(records.all { it.recordTimestamp in 1..4102444800L && it.recordIndex >= 0 }) { "record timestamp/index invalid; use Unix seconds" }
        val action: suspend (ServerClient) -> JsonElement = { c ->
            val r = upload(c.getDataService(), records)
            if (!r.isSuccess()) throw IllegalStateException("server rejected upload")
            obj("status" to str("accepted"), "count" to number(records.size), "resync" to JsonPrimitive(false), "verification" to str("Read back before claiming records match."))
        }; action
    }
}

fun registerTools() {
    dataTools("short_summaries", ShortSummaryNetwork.serializer(),
        { d, a -> d.downloadShortSummaries(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(ShortSummaryNetwork.serializer()), r as DownloadResponse<ShortSummaryNetwork>) },
        { d, records -> d.uploadShortSummaries(records, resync = false) }, full = false, events = false)
    dataTools("pressures", BloodPressureNetwork.serializer(),
        { d, a -> d.downloadPressures(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(BloodPressureNetwork.serializer()), r as DownloadResponse<BloodPressureNetwork>) },
        { d, records -> d.uploadPressures(records, resync = false) }, full = false, events = false)
    dataTools("full_summaries", FullSummaryNetwork.serializer(),
        { d, a -> d.downloadFullSummaries(fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(FullSummaryNetwork.serializer()), r as DownloadResponse<FullSummaryNetwork>) },
        { d, records -> d.uploadFullSummaries(records, resync = false) }, full = true, events = false)
    dataTools("events", EventNetwork.serializer(),
        { d, a -> d.downloadEvents(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId"), moduleId = a.long("moduleId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(EventNetwork.serializer()), r as DownloadResponse<EventNetwork>) },
        { d, records -> d.uploadEvents(records, resync = false) }, full = false, events = true)
    dataTools("sleep_events", EventNetwork.serializer(),
        { d, a -> d.downloadSleepEvents(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(EventNetwork.serializer()), r as DownloadResponse<EventNetwork>) },
        { d, records -> d.uploadSleepEvents(records.map { SleepEventNetwork(it.recordIndex, it.recordTimestamp, it.utcOffset, it.eventType, it.eventData, it.sensorId) }, resync = false) }, full = false, events = false)
    dataTools("system_events", EventNetwork.serializer(),
        { d, a -> d.downloadSystemEvents(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(EventNetwork.serializer()), r as DownloadResponse<EventNetwork>) },
        { d, records -> d.uploadSystemEvents(records.map { SystemEventNetwork(it.recordIndex, it.recordTimestamp, it.utcOffset, it.eventType, it.eventData, it.sensorId) }, resync = false) }, full = false, events = false)
    dataTools("sleep_infos", SleepInfoNetwork.serializer(),
        { d, a -> d.downloadSleepInfos(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(SleepInfoNetwork.serializer()), r as DownloadResponse<SleepInfoNetwork>) },
        { d, records -> d.uploadSleepInfos(records, resync = false) }, full = false, events = false)
    dataTools("meals", MealNetwork.serializer(),
        { d, a -> d.downloadMeals(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(MealNetwork.serializer()), r as DownloadResponse<MealNetwork>) },
        { d, records -> d.uploadMeals(records, resync = false) }, full = false, events = false)
    dataTools("anxieties", AnxietyNetwork.serializer(),
        { d, a -> d.downloadAnxieties(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(AnxietyNetwork.serializer()), r as DownloadResponse<AnxietyNetwork>) },
        { d, records -> d.uploadAnxieties(records, resync = false) }, full = false, events = false)
    dataTools("resync_ranges", ResyncRangeNetwork.serializer(),
        { d, a -> d.downloadResyncRanges(from = a.long("from"), fromTime = a.long("fromTime"), toTime = a.long("toTime"), limit = a.long("limit") ?: 50, sensorId = a.text("sensorId")) },
        { r -> @Suppress("UNCHECKED_CAST") json.encodeToJsonElement(DownloadResponse.serializer(ResyncRangeNetwork.serializer()), r as DownloadResponse<ResyncRangeNetwork>) },
        { d, records -> d.uploadResyncRanges(records, resync = false) }, full = false, events = false)
    for (mode in listOf("measurement_time", "last_update")) {
        tools += Tool("download_weights_by_$mode", "Download weights from Unix seconds. In serverApi 1.6.0 BOTH methods filter measurement time, not last-update time. Server ignores limit; wrapper returns offset/limit slice with has_more. Read back weight AND body composition: add may succeed with enrichment missing.", inputs(mapOf("fromTime" to scalar("integer", "Measurement time lower bound in Unix seconds."), "limit" to scalar("integer", "Returned slice size 1..100, default 50."), "offset" to scalar("integer", "Offset into the server result, default 0.")), listOf("fromTime")), true) { a ->
            val start = a.long("fromTime") ?: error("fromTime required")
            val limit = a.long("limit") ?: 50
            val offset = a.long("offset") ?: 0
            require(start in 0..4102444800L && limit in 1..100 && offset in 0..Int.MAX_VALUE) { "invalid time, limit or offset" }
            val action: suspend (ServerClient) -> JsonElement = { c ->
                val service = c.getWeightService()
                val all = if (mode == "measurement_time") service.getWeightsByMeasurementTime(Instant.fromEpochSeconds(start)) else service.getWeightsByLastUpdate(Instant.fromEpochSeconds(start))
                val page = all.drop(offset.toInt()).take(limit.toInt())
                obj("records" to JsonArray(page.map(::weightOutput)), "total" to number(all.size), "offset" to number(offset), "next_offset" to number(offset+page.size), "has_more" to JsonPrimitive(offset+page.size < all.size))
            }; action
        }
    }
    for (mode in listOf("add", "update", "add_batch")) {
        val batch = mode == "add_batch"
        val props = if (batch) mapOf("data" to arraySchema(schema(WeightInput.serializer().descriptor))) else mapOf("data" to schema(WeightInput.serializer().descriptor))
        tools += Tool("${mode}_weight", "${mode.replace('_',' ')} synthetic weight in kilograms, measurementTime in Unix SECONDS. Optional body composition includes muscleMass in kg. source defaults HEALBE. Batch stops at first error, retaining prior successes. ID required only for update. No deletion. Add success confirms the weighing only: SDK enrichment is best effort; download and compare composition. Do not retry uncertain writes.", inputs(props, listOf("data")), false) { a ->
            val records = if (batch) json.decodeFromJsonElement<List<WeightInput>>(a.getValue("data")) else listOf(json.decodeFromJsonElement<WeightInput>(a.getValue("data")))
            records.forEach { it.check(mode == "update") }
            require(records.map { it.measurementTime }.distinct().size == records.size) { "duplicate measurementTime within batch" }
            val action: suspend (ServerClient) -> JsonElement = { c ->
                val outcomes = mutableListOf<JsonElement>()
                for ((index, w) in records.withIndex()) {
                    try {
                        val result = if (mode == "update") json.encodeToJsonElement(c.getWeightService().updateWeight(w)) else json.encodeToJsonElement(c.getWeightService().addNewWeight(w))
                        outcomes += obj("index" to number(index), "status" to str("accepted"), "measurementTime" to number(w.measurementTime), "result" to result)
                    } catch (e: Exception) {
                        outcomes += obj("index" to number(index), "status" to str("failed_or_unknown"), "errorType" to str(e.javaClass.simpleName))
                        break
                    }
                }
                obj("outcomes" to JsonArray(outcomes), "requested" to number(records.size), "attempted" to number(outcomes.size), "accepted" to number(outcomes.count { it.jsonObject.text("status") == "accepted" }), "verification" to str("Read back measurementTime, weight and composition. Do not replay failed_or_unknown writes without reconciliation."))
            }; action
        }
    }
}

fun safeError(message: String) = obj("content" to JsonArray(listOf(obj("type" to str("text"), "text" to str(message)))), "isError" to JsonPrimitive(true))
fun callTool(params: JsonObject): JsonObject = runBlocking {
    val tool = tools.find { it.name == params.text("name") } ?: return@runBlocking safeError("Unknown tool")
    val args = params["arguments"] as? JsonObject ?: return@runBlocking safeError("arguments must be an object")
    val prepared = try {
        validate(args, tool.input)
        tool.prepare(args)
    } catch (e: Exception) { return@runBlocking safeError("Invalid input: " + if (e is IllegalArgumentException && e !is SerializationException) e.message else e.javaClass.simpleName) }
    val supplied = args.containsKey("login") || args.containsKey("password")
    val login = if (supplied) args.text("login") else System.getenv("HEALBE_LOGIN")
    val password = if (supplied) args.text("password") else System.getenv("HEALBE_PASSWORD")
    if (login.isNullOrBlank() || password.isNullOrEmpty()) return@runBlocking safeError("Supply both login and password, or configure HEALBE_LOGIN/HEALBE_PASSWORD privately.")
    val access = InMemoryAccessProvider()
    val client = ServerClient(access, "Week6-Task4-MCP/1.0")
    System.getenv("HEALBE_ENDPOINT")?.let { client.endpoint = it }
    try {
        val auth = client.getUserService().authUser(login, password)
        if (!auth.isSuccess() || auth.accessId.isNullOrBlank()) return@runBlocking safeError("Authentication failed; no data operation attempted.")
        access.saveAccessId(auth)
    } catch (e: Exception) { return@runBlocking safeError("Authentication failed (${e.javaClass.simpleName}); no data operation attempted.") }
    try {
        val result = prepared(client).toString()
        if (result.toByteArray().size > 28000) return@runBlocking safeError("Response exceeds 28000 bytes. Reduce download limit. Do not repeat uploads.")
        obj("content" to JsonArray(listOf(obj("type" to str("text"), "text" to str(result)))))
    } catch (e: Exception) {
        safeError("Operation failed (${e.javaClass.simpleName}). ${if (tool.readOnly) "No writes attempted." else "Write outcome may be unknown; read back before retrying."}")
    } finally { access.clearAccessId() }
}

fun main() {
    Logger.clearImplementations()
    registerTools()
    var initialized = false
    val input = System.`in`.bufferedReader(Charsets.UTF_8)
    val output = System.out.bufferedWriter(Charsets.UTF_8)
    fun respond(id: JsonElement, result: JsonElement? = null, code: Int = -32600, message: String = "Invalid request") {
        val response = if (result != null) obj("jsonrpc" to str("2.0"), "id" to id, "result" to result) else obj("jsonrpc" to str("2.0"), "id" to id, "error" to obj("code" to number(code), "message" to str(message)))
        output.write(response.toString()); output.newLine(); output.flush()
    }
    while (true) {
        val line = input.readLine() ?: break
        val req = try { json.parseToJsonElement(line).jsonObject } catch (_: Exception) { respond(JsonNull, code = -32700, message = "Parse error"); continue }
        val id = req["id"] ?: continue // Notifications have no response.
        val method = req.text("method")
        val params = req["params"] as? JsonObject ?: JsonObject(emptyMap())
        if (req.text("jsonrpc") != "2.0") { respond(id); continue }
        when {
            method == "initialize" && !initialized -> {
                initialized = true
                respond(id, obj("protocolVersion" to str("2024-11-05"), "capabilities" to obj("tools" to obj()), "serverInfo" to obj("name" to str("healbe-data-mcp"), "version" to str("1.0.0"))))
            }
            method == "ping" -> respond(id, obj())
            !initialized -> respond(id, code = -32000, message = "Initialize first")
            method == "tools/list" -> respond(id, obj("tools" to JsonArray(tools.map { it.definition() })))
            method == "tools/call" -> respond(id, callTool(params))
            else -> respond(id, code = -32601, message = "Method not found")
        }
    }
    // ServerClient has no close API; release Ktor/OkHttp workers when the stdio client exits.
    kotlin.system.exitProcess(0)
}
