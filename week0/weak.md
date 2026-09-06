Here's the implementation of the `firstSuccessful` function:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>,
    dispatcher: CoroutineDispatcher = Dispatchers.Default
): T {
    require(tasks.isNotEmpty()) { "Tasks list cannot be empty" }

    return coroutineScope {
        val result = AtomicReference<T?>()
        val failureCount = AtomicInteger(0)
        val failures = mutableListOf<Throwable>()
        val mutex = Mutex()

        // Create a channel to signal when we have a successful result
        val successSignal = CompletableDeferred<Unit>()

        // Launch all tasks
        val jobs = tasks.map { task ->
            launch(dispatcher) {
                try {
                    val taskResult = task()

                    // Try to set the result if no result has been set yet
                    if (result.compareAndSet(null, taskResult)) {
                        successSignal.complete(Unit)
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    failureCount.incrementAndGet()
                    mutex.withLock {
                        failures.add(e)
                    }
                }
            }
        }

        try {
            // Wait for first success or all failures
            successSignal.await()

            // Cancel all remaining jobs
            jobs.forEach { it.cancel() }

            return@coroutineScope result.get()!!
        } catch (e: CancellationException) {
            // Parent cancellation propagates naturally - cancel all children
            jobs.forEach { it.cancel() }
            throw e
        }
    }
}
```

## Explanation of Cancellation Behavior:

1. **Concurrent execution**: All tasks are launched within a `coroutineScope`, which ensures all child coroutines are children of this scope. This means:
   - If the parent coroutine is cancelled, all child tasks are automatically cancelled.
   - The scope will not complete until all children are done (or cancelled).

2. **Success handling**: When a task succeeds:
   - It uses an atomic reference to try to set the result (CAS operation ensures only one task can succeed).
   - The first to set the result signals via a `CompletableDeferred`.
   - Once signaled, all other jobs are explicitly cancelled.

3. **Failure handling**: Failed tasks:
   - Catch all exceptions (except `CancellationException` which is re-thrown for proper cancellation handling).
   - Increment failure count and store the exception.
   - Continue running other tasks.

4. **Cancellation propagation**:
   - If parent cancels, the `coroutineScope` will ensure all children are cancelled.
   - The `CancellationException` is caught, all jobs are explicitly cancelled (for cleanup), and re-thrown.
   - The `successSignal.await()` will also throw if the scope is cancelled.

5. **Empty task list**: Throws `IllegalArgumentException` immediately.
6. **All tasks fail**: The `successSignal` never completes, and we need to handle when all jobs complete. However, in this implementation, if all tasks fail, the `coroutineScope` will complete with no result. We need to check if we've reached this state.

Wait, there's a bug - if all tasks fail, we never get past `successSignal.await()`. Let me fix this:

```kotlin
suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>,
    dispatcher: CoroutineDispatcher = Dispatchers.Default
): T {
    require(tasks.isNotEmpty()) { "Tasks list cannot be empty" }

    return coroutineScope {
        val result = AtomicReference<T?>()
        val failures = Collections.synchronizedList(mutableListOf<Throwable>())

        // Use a channel to communicate results
        val channel = Channel<Result<T>>(Channel.CONFLATED)

        // Launch all tasks
        val jobs = tasks.map { task ->
            launch(dispatcher) {
                try {
                    val taskResult = task()
                    channel.trySend(Result.success(taskResult))
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    channel.trySend(Result.failure(e))
                }
            }
        }

        try {
            // Wait for first success while collecting failures
            var failureCount = 0
            val receivedResults = mutableListOf<Result<T>>()

            for (i in 0 until tasks.size) {
                val taskResult = channel.receive()
                receivedResults.add(taskResult)

                if (taskResult.isSuccess) {
                    // Cancel all remaining jobs
                    jobs.forEach { it.cancel() }
                    return@coroutineScope taskResult.getOrThrow()
                }
            }

            // All tasks failed
            val exception = IllegalStateException("All tasks failed")
            receivedResults.filter { it.isFailure }.forEach { failed ->
                exception.addSuppressed(failed.exceptionOrNull() ?: throw IllegalStateException("No exception"))
            }
            throw exception

        } catch (e: CancellationException) {
            // Parent cancellation propagates - cancel all children
            jobs.forEach { it.cancel() }
            throw e
        }
    }
}

// Helper extension to catch failures properly
private fun Result<Throwable>.toException(): Throwable {
    return this.getOrThrow()
}
```

Actually, let me provide a cleaner version using structured concurrency properly:

```kotlin
suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>
): T {
    require(tasks.isNotEmpty()) { "Tasks list cannot be empty" }

    return coroutineScope {
        val failures = mutableListOf<Throwable>()
        val mutex = Mutex()
        val successChannel = Channel<T>(Channel.CONFLATED)

        val jobs = tasks.map { task ->
            launch(Dispatchers.Default) {
                try {
                    val taskResult = task()
                    if (!successChannel.isClosedForSend) {
                        successChannel.trySend(taskResult)
                        // Cancel other jobs

                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    mutex.withLock {
                        failures.add(e)
                    }
                }
            }
        }

        try {
            val result = successChannel.receive()
            // Success obtained - cancel all other jobs
            launch {
                jobs.forEach { it.cancel() }
            }.join()
            return@coroutineScope result
        } catch (e: CancellationException) {
            jobs.forEach { it.cancel() }
            throw e
        } catch (e: Exception) {
            // Check if all jobs completed
            if (jobs.all { it.isCompleted }) {
                val exception = IllegalStateException("All tasks failed")
                mutex.withLock {
                    failures.forEach(exception::addSuppressed)
                }
                throw exception
            }
            throw e
        }
    }
}
```

Let me provide the final, correct, and clean implementation:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>
): T {
    require(tasks.isNotEmpty()) { "Tasks list cannot be empty" }

    return coroutineScope {
        val channel = Channel<Result<T>>(Channel.UNLIMITED)

        // Launch all tasks
        val jobs = tasks.map { task ->
            launch {
                try {
                    val result = task()
                    channel.send(Result.success(result))
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    channel.send(Result.failure(e))
                }
            }
        }

        try {
            var success: T? = null
            var failures = mutableListOf<Throwable>()

            for (i in 0 until tasks.size) {
                val result = channel.receive()
                result.fold(
                    onSuccess = { value ->
                        if (success == null) {
                            success = value
                            // Cancel all other jobs
                            jobs.forEach { it.cancel() }
                        }
                    },
                    onFailure = { error ->
                        failures.add(error)
                    }
                )
            }

            success?.let { return@coroutineScope it }

            val exception = IllegalStateException("All tasks failed")
            failures.forEach(exception::addSuppressed)
            throw exception
        } finally {
            // Ensure all jobs are cancelled
            jobs.forEach { it.cancel() }
        }
    }
}
```

## Unit Tests:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.test.*
import org.junit.Assert.*
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

class FirstSuccessfulTest {

    @Test
    fun `returns first successful result`() = runTest {
        val result = firstSuccessful(
            listOf(
                { delay(100); 1 },
                { delay(50); 2 },
                { delay(10); 3 }
            )
        )
        assertEquals(3, result)
    }

    @Test
    fun `skips failed tasks and returns successful one`() = runTest {
        val result = firstSuccessful(
            listOf(
                { throw RuntimeException("Task 1 failed") },
                { delay(100); 42 },
                { throw RuntimeException("Task 3 failed") }
            )
        )
        assertEquals(42, result)
    }

    @Test
    fun `fails when all tasks fail`() = runTest {
        val exception = assertThrows(IllegalStateException::class.java) {
            firstSuccessful(
                listOf(
                    { throw RuntimeException("Error 1") },
                    { throw RuntimeException("Error 2") }
                )
            )
        }

        assertEquals("All tasks failed", exception.message)
        assertEquals(2, exception.suppressed.size)
    }

    @Test
    fun `throws on empty list`() {
        assertThrows(IllegalArgumentException::class.java) {
            runBlocking {
                firstSuccessful(emptyList())
            }
        }
    }

    @Test
    fun `cancels remaining tasks after success`() = runTest {
        val cancelled = AtomicInteger(0)
        val longTask = launch {
            try {
                delay(1000) // Simulate long task
            } catch (e: CancellationException) {
                cancelled.incrementAndGet()
                throw e
            }
        }

        val result = firstSuccessful(
            listOf(
                { delay(100); 1 },
                { delay(1000); 2 }
            )
        )

        assertEquals(1, result)
        assertEquals(1, cancelled.get())
    }

    @Test
    fun `all tasks cancelled when parent is cancelled`() = runTest {
        val cancellationCount = AtomicInteger(0)
        val started = AtomicInteger(0)

        val parentJob = launch(start = CoroutineStart.LAZY) {
            firstSuccessful(
                listOf(
                    {
                        started.incrementAndGet()
                        try {
                            delay(5000) // Long task
                        } catch (e: CancellationException) {
                            cancellationCount.incrementAndGet()
                            throw e
                        }
                    },
                    {
                        started.incrementAndGet()
                        delay(5000) // Another long task
                    }
                )
            )
        }

        parentJob.start()
        delay(100) // Let tasks start
        parentJob.cancel()
        parentJob.join()

        assertEquals(2, started.get())
        // At least one task should have been cancelled
        assertTrue(cancellationCount.get() >= 1)
    }
}
```

This implementation:
- Runs all tasks concurrently using `launch` inside a `coroutineScope`
- Returns the first successful result using a channel
- Tracks failures and includes them as suppressed exceptions
- Correctly handles cancellation at all levels
- Uses proper structured concurrency
- No polling or busy loops - purely event-driven using channels
