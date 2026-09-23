import com.espertech.esper.common.client.EPCompiled;
import com.espertech.esper.common.client.EventBean;
import com.espertech.esper.common.client.configuration.Configuration;
import com.espertech.esper.common.client.configuration.common.ConfigurationCommonEventTypeXMLDOM;
import com.espertech.esper.common.client.fireandforget.EPFireAndForgetPreparedQueryParameterized;
import com.espertech.esper.common.client.hook.exception.ExceptionHandler;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactory;
import com.espertech.esper.common.client.hook.exception.ExceptionHandlerFactoryContext;
import com.espertech.esper.common.client.json.minimaljson.Json;
import com.espertech.esper.common.client.json.minimaljson.JsonArray;
import com.espertech.esper.common.client.json.minimaljson.JsonNumber;
import com.espertech.esper.common.client.json.minimaljson.JsonObject;
import com.espertech.esper.common.client.json.minimaljson.JsonString;
import com.espertech.esper.common.client.json.minimaljson.JsonValue;
import com.espertech.esper.common.client.json.minimaljson.Member;
import com.espertech.esper.common.client.util.UndeployRethrowPolicy;
import com.espertech.esper.compiler.client.CompilerArguments;
import com.espertech.esper.compiler.client.EPCompileException;
import com.espertech.esper.compiler.client.EPCompilerProvider;
import com.espertech.esper.runtime.client.DeploymentOptions;
import com.espertech.esper.runtime.client.EPDeployException;
import com.espertech.esper.runtime.client.EPDeployment;
import com.espertech.esper.runtime.client.EPRuntime;
import com.espertech.esper.runtime.client.EPRuntimeProvider;
import com.espertech.esper.runtime.client.EPStatement;
import com.espertech.esper.runtime.client.UpdateListener;

import org.w3c.dom.Document;
import org.xml.sax.InputSource;

import javax.xml.parsers.DocumentBuilderFactory;
import java.io.StringReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Java oracle for EPLContainedEventExample ordinals 0 through 5: the
 * media-order XML contained-event example. Six executions on six runtimes.
 *
 * Ord 0 (EPLContainedExample) deploys twelve statements over the MediaOrder
 * XML DOM type: s1 selects orderId plus the indexed items.item[0].itemId
 * path, s2 expands [books.book], s3 filters the parent before expansion,
 * s4 counts over #unique(bookId), s5 expands [books.book][review], s6 is a
 * never-fired Cancel->book pattern, s7-s10 exercise contained select
 * projections (parent fields, wildcard fragments, grandparent references),
 * s11_0 inserts fragment-typed columns into ReviewStream and s11 reads them
 * back, and s12 filters the book fragment before expanding review. One
 * mediaOrderOne.xml send fires every statement except s6.
 *
 * Ord 1 (EPLContainedSolutionPattern) expands the SupportResponseEvent
 * .subEvents bean array inside a contained select that carries the parent
 * category, windows the fragments over #time(1 min) and groups by
 * category+subEventType; two sends yield cumulative averages.
 *
 * Ords 2-4 self-join the book and item contained streams on
 * productId=bookId (inner, left-outer, full-outer) in three phases: the
 * row-select phase, a cumulative count(*) phase and a unidirectional
 * count(*) phase, each separated by undeployAll.
 *
 * Ord 5 (EPLContainedSolutionPatternFinancial) deploys one module: Symbol
 * and bus-event ForeignSymbols/LocalSymbols map schemas, the two-key
 * Mapping table with secondary indexes, the SymbolsPair lastevent join
 * insert, the split-all on-trigger that emits begin marker, contained
 * foreign/local company rows, output marker and end marker, the
 * start/end-event SymbolsPairContext, the contexted Result table, the two
 * contexted merges with scalar Mapping subselects, and the ordered 'out'
 * select. Four fire-and-forget positional-parameter inserts load Mapping;
 * ForeignSymbols/LocalSymbols sends then produce the four ordered rows.
 *
 * The XML DOM type registration mirrors TestSuiteEPLContained.configure:
 * MediaOrder and Cancel share one ConfigurationCommonEventTypeXMLDOM whose
 * schemaResource is the checkout's regression/mediaOrderSchema.xsd (resolved
 * from the -DesperRoot system property) and whose rootElementName is
 * "mediaorder". SupportResponseEvent/SupportResponseSubEvent are local
 * public static beans mirroring the regression beans (regression-lib is not
 * on the oracle classpath).
 */
public final class EPLContainedEventExampleScenarioOracle {
    private static final String VERSION = "esper-parity/v1";
    private static final String ID = "epl-contained-event-example";
    private static final String DESCRIPTION =
            "EPLContainedEventExample ords 0-5: the media-order XML contained-event "
                    + "example. Ord 0 expands MediaOrder[books.book] and "
                    + "[books.book][review] contained fragments (s1 indexed item "
                    + "path, s2 wildcard book, s3 filtered parent, s4 unique-count, "
                    + "s5 review fields, s6 unobserved pattern, s7-s10 contained "
                    + "select projections, s11 ReviewStream round-trip, s12 filtered "
                    + "book). Ord 1 expands the SupportResponseEvent.subEvents bean "
                    + "array under a 1-minute time window grouped by parent category. "
                    + "Ords 2-4 self-join the book and item contained streams on "
                    + "productId=bookId (inner, left-outer, full-outer) plus "
                    + "cumulative and unidirectional count phases. Ord 5 is the "
                    + "financial solution pattern: a SymbolsPair lastevent join feeds "
                    + "a split-all on-trigger that unnests foreign/local companies, "
                    + "merges foreign and local Symbol rows into the contexted Result "
                    + "table via Mapping subselects, and fires the ordered "
                    + "SymbolsPairOutputEvent select between begin/end markers (Java "
                    + "source regression-lib/src/main/java/com/espertech/esper/"
                    + "regressionlib/suite/epl/contained/EPLContainedEventExample.java).";
    private static final String JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c";
    private static final String JAVA_SOURCE =
            "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/contained/"
                    + "EPLContainedEventExample.java";

    private static final String[] RUNTIME_IDS = {
            "java-runtime-36bfa282c286614e1bc0",
            "java-runtime-3bcf56da38fd1e2f669a",
            "java-runtime-06cbcd9c14891d14a995",
            "java-runtime-bfcd3e5954bd02726c90",
            "java-runtime-e9cfcbfcc5556a9ea9ab",
            "java-runtime-23292c0fd335038c05d4"
    };
    private static final String[] EXECUTION_NAMES = {
            "EPLContainedExample",
            "EPLContainedSolutionPattern",
            "EPLContainedJoinSelfJoin",
            "EPLContainedJoinSelfLeftOuterJoin",
            "EPLContainedJoinSelfFullOuterJoin",
            "EPLContainedSolutionPatternFinancial"
    };
    private static final String[] STATIC_IDS = {
            "java-d763dc53250784007213",
            "java-3370a9fc6c79a91c99db",
            "java-a340361a64068355d364",
            "java-e0629eece325f19d5eab",
            "java-b66b814fdafb047c88ca",
            "java-364de601500af6cb9ce9"
    };
    private static final String[] JAVA_FLAGS = {"FIREANDFORGET"};
    private static final String[] CASES = {
            "contained-example",
            "solution-pattern",
            "join-self-join",
            "join-self-left-outer",
            "join-self-full-outer",
            "financial"
    };
    private static final int[] ORDINALS = {0, 1, 2, 3, 4, 5};
    private static final String[] CASE_OBSERVATIONS = {
            "listener; twelve deploys over the MediaOrder XML DOM type: s1 "
                    + "orderId plus indexed items.item[0].itemId, s2 wildcard book "
                    + "fragments, s3 filtered parent expansion, s4 count over "
                    + "#unique(bookId), s5 review fragments, s6 unobserved "
                    + "Cancel->book pattern, s7-s10 contained select projections, "
                    + "s11 ReviewStream round-trip, s12 filtered book expansion; "
                    + "one mediaOrderOne.xml send fires every statement except s6",
            "listener; one deploy expands the SupportResponseEvent.subEvents bean "
                    + "array under #time(1 min) grouped by category+subEventType; "
                    + "two bean sends yield cumulative averages "
                    + "{svcOne,typeA,1000},{svcOne,typeB,800} then "
                    + "{svcOne,typeA,750},{svcOne,typeB,600}",
            "listener; three phases separated by undeploy-all: the book/item "
                    + "inner join row select, a cumulative count(*) join and a "
                    + "unidirectional count(*) join; sends docOne, docOne, docTwo, "
                    + "then docTwo, docOne, then docTwo, docOne",
            "listener; three phases separated by undeploy-all: the book/item "
                    + "left-outer join row select, a cumulative count(*) join and "
                    + "a unidirectional count(*) join; sends docTwo, docOne, then "
                    + "docTwo, docOne, then docTwo, docOne",
            "listener; three phases separated by undeploy-all: the book/item "
                    + "full-outer join row select (item side carries the contained "
                    + "select orderId), a cumulative count(*) join and a "
                    + "unidirectional count(*) join; sends docTwo, docOne, then "
                    + "docTwo, docOne, then docTwo, docOne",
            "listener; one module deploy registers map schemas, the Mapping table "
                    + "with secondary indexes, the SymbolsPair lastevent insert, "
                    + "the begin/foreign/local/output/end split-all with contained "
                    + "branches, the start/end-event context, the contexted Result "
                    + "table merges with Mapping subselects and the ordered out "
                    + "select; four fire-and-forget Mapping loads seed the pairs; "
                    + "ForeignSymbols/LocalSymbols sends produce four ordered rows"
    };

    // Verbatim transcriptions of EPLContainedEventExample lines 156-194 (ord 0),
    // 371 (ord 1), 225/244/255 (ord 2), 278/294/305 (ord 3), 328/344/355 (ord 4)
    // and 64-95 (ord 5 module). @name('sN') prefixes carry a space before select.
    private static final String EPL_ORD0_S1 =
            "@name('s1') select orderId, items.item[0].itemId from MediaOrder";
    private static final String EPL_ORD0_S2 =
            "@name('s2') select * from MediaOrder[books.book]";
    private static final String EPL_ORD0_S3 =
            "@name('s3') select * from MediaOrder(orderId='PO200901')[books.book]";
    private static final String EPL_ORD0_S4 =
            "@name('s4') select count(*) from MediaOrder[books.book]#unique(bookId)";
    private static final String EPL_ORD0_S5 =
            "@name('s5') select * from MediaOrder[books.book][review]";
    private static final String EPL_ORD0_S6 =
            "@name('s6') select * from pattern [c=Cancel -> o=MediaOrder(orderId = c.orderId)[books.book]]";
    private static final String EPL_ORD0_S7 =
            "@name('s7') select * from MediaOrder[select orderId, bookId from books.book][select * from review]";
    private static final String EPL_ORD0_S8 =
            "@name('s8') select * from MediaOrder[select * from books.book][select reviewId, comment from review]";
    private static final String EPL_ORD0_S9 =
            "@name('s9') select * from MediaOrder[books.book as book][select book.*, reviewId, comment from review]";
    private static final String EPL_ORD0_S10 =
            "@name('s10') select * from MediaOrder[books.book as book][select mediaOrder.*, bookId, reviewId from review] as mediaOrder";
    private static final String EPL_ORD0_S11_0 =
            "@name('s11_0') @public insert into ReviewStream select * from MediaOrder[books.book as book]\n"
                    + "    [select mediaOrder.* as mediaOrder, book.* as book, review.* as review from review as review] as mediaOrder";
    private static final String EPL_ORD0_S11 =
            "@name('s11') select mediaOrder.orderId, book.bookId, review.reviewId from ReviewStream";
    private static final String EPL_ORD0_S12 =
            "@name('s12') select * from MediaOrder[books.book where author = 'Orson Scott Card'][review]";

    private static final String EPL_ORD1_S0 =
            "@name('s0') select category, subEventType, avg(responseTimeMillis) as avgTime "
                    + "from SupportResponseEvent[select category, * from subEvents]#time(1 min) "
                    + "group by category, subEventType order by category, subEventType";

    private static final String EPL_JOIN_ROWS_INNER =
            "@name('s0') select book.bookId,item.itemId from MediaOrder[books.book] as book, "
                    + "MediaOrder[items.item] as item where productId = bookId "
                    + "order by bookId, item.itemId asc";
    private static final String EPL_JOIN_COUNT_INNER =
            "@name('s0') select count(*) from MediaOrder[books.book] as book, "
                    + "MediaOrder[items.item] as item where productId = bookId order by bookId asc";
    private static final String EPL_JOIN_UNI_INNER =
            "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional, "
                    + "MediaOrder[items.item] as item where productId = bookId order by bookId asc";
    private static final String EPL_JOIN_ROWS_LEFT =
            "@name('s0') select book.bookId,item.itemId from MediaOrder[books.book] as book "
                    + "left outer join MediaOrder[items.item] as item on productId = bookId "
                    + "order by bookId, item.itemId asc";
    private static final String EPL_JOIN_COUNT_LEFT =
            "@name('s0') select count(*) from MediaOrder[books.book] as book "
                    + "left outer join MediaOrder[items.item] as item on productId = bookId";
    private static final String EPL_JOIN_UNI_LEFT =
            "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional "
                    + "left outer join MediaOrder[items.item] as item on productId = bookId";
    private static final String EPL_JOIN_ROWS_FULL =
            "@name('s0') select orderId, book.bookId,item.itemId from MediaOrder[books.book] as book "
                    + "full outer join MediaOrder[select orderId, * from items.item] as item "
                    + "on productId = bookId order by bookId, item.itemId asc";
    private static final String EPL_JOIN_COUNT_FULL =
            "@name('s0') select count(*) from MediaOrder[books.book] as book "
                    + "full outer join MediaOrder[items.item] as item on productId = bookId";
    private static final String EPL_JOIN_UNI_FULL =
            "@name('s0') select count(*) from MediaOrder[books.book] as book unidirectional "
                    + "full outer join MediaOrder[items.item] as item on productId = bookId";

    private static final String EPL_FINANCIAL_MODULE =
            "create schema Symbol(symbol string, value double);\n"
                    + "@public @buseventtype create schema ForeignSymbols(companies Symbol[]);\n"
                    + "@public @buseventtype create schema LocalSymbols(companies Symbol[]);\n"
                    + "\n"
                    + "@public create table Mapping(foreignSymbol string primary key, localSymbol string primary key);\n"
                    + "create index MappingIndexForeignSymbol on Mapping(foreignSymbol);\n"
                    + "create index MappingIndexLocalSymbol on Mapping(localSymbol);\n"
                    + "\n"
                    + "insert into SymbolsPair select * from ForeignSymbols#lastevent as foreign, LocalSymbols#lastevent as local;\n"
                    + "on SymbolsPair\n"
                    + "  insert into SymbolsPairBeginEvent select null\n"
                    + "  insert into ForeignSymbolRow select * from [foreign.companies]\n"
                    + "  insert into LocalSymbolRow select * from [local.companies]\n"
                    + "  insert into SymbolsPairOutputEvent select null"
                    + "  insert into SymbolsPairEndEvent select null"
                    + "  output all;\n"
                    + "\n"
                    + "create context SymbolsPairContext start SymbolsPairBeginEvent end SymbolsPairEndEvent;\n"
                    + "context SymbolsPairContext create table Result(foreignSymbol string primary key, localSymbol string primary key, value double);\n"
                    + "\n"
                    + "context SymbolsPairContext on ForeignSymbolRow as fsr merge Result as result where result.foreignSymbol = fsr.symbol\n"
                    + "  when not matched then insert select fsr.symbol as foreignSymbol,\n"
                    + "    (select localSymbol from Mapping as mapping where mapping.foreignSymbol = fsr.symbol) as localSymbol, fsr.value as value\n"
                    + "  when matched and fsr.value > result.value then update set value = fsr.value;\n"
                    + "\n"
                    + "context SymbolsPairContext on LocalSymbolRow as lsr merge Result as result where result.localSymbol = lsr.symbol\n"
                    + "  when not matched then insert select (select foreignSymbol from Mapping as mapping where mapping.localSymbol = lsr.symbol) as foreignSymbol,"
                    + "    lsr.symbol as localSymbol, lsr.value as value\n"
                    + "  when matched and lsr.value > result.value then update set value = lsr.value;\n"
                    + "\n"
                    + "@name('out') context SymbolsPairContext on SymbolsPairOutputEvent select foreignSymbol, localSymbol, value from Result order by foreignSymbol asc;\n";
    private static final String EPL_FAF_MAPPING =
            "insert into Mapping select ?::string as foreignSymbol, ?::string as localSymbol";

    private static final int EXPECTED_STEPS = 70;
    private static final int EXPECTED_RECORDS = 33;

    private static final String MEDIA_ORDER_ONE =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
                    + "<mediaorder xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:noNamespaceSchemaLocation=\"mediaOrderSchema.xsd\">\n"
                    + "  <orderId>PO200901</orderId>\n"
                    + "  <items>\n"
                    + "    <item>\n"
                    + "      <itemId>100001</itemId>\n"
                    + "      <productId>B001</productId>\n"
                    + "      <amount>10</amount>\n"
                    + "      <price>11.95</price>\n"
                    + "    </item>\n"
                    + "  </items>\n"
                    + "  <books>\n"
                    + "    <book>\n"
                    + "      <bookId>B001</bookId>\n"
                    + "      <author>Orson Scott Card</author>\n"
                    + "      <review>\n"
                    + "        <reviewId>1</reviewId>\n"
                    + "        <comment>best book ever</comment>\n"
                    + "      </review>\n"
                    + "    </book>\n"
                    + "    <book>\n"
                    + "      <bookId>B002</bookId>\n"
                    + "      <author>Isaac Asimov</author>\n"
                    + "    </book>\n"
                    + "  </books>\n"
                    + "</mediaorder>\n";
    private static final String MEDIA_ORDER_TWO =
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
                    + "<mediaorder xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:noNamespaceSchemaLocation=\"mediaOrderSchema.xsd\">\n"
                    + "  <orderId>PO200901</orderId>\n"
                    + "  <items>\n"
                    + "    <item>\n"
                    + "      <itemId>200001</itemId>\n"
                    + "      <productId>B006</productId>\n"
                    + "      <amount>7</amount>\n"
                    + "      <price>7.45</price>\n"
                    + "    </item>\n"
                    + "    <item>\n"
                    + "      <itemId>200002</itemId>\n"
                    + "      <productId>B005</productId>\n"
                    + "      <amount>1</amount>\n"
                    + "      <price>67.99</price>\n"
                    + "    </item>\n"
                    + "    <item>\n"
                    + "      <itemId>200003</itemId>\n"
                    + "      <productId>B007</productId>\n"
                    + "      <amount>0</amount>\n"
                    + "      <price>0</price>\n"
                    + "    </item>\n"
                    + "    <item>\n"
                    + "      <itemId>200004</itemId>\n"
                    + "      <productId>B005</productId>\n"
                    + "      <amount>2</amount>\n"
                    + "      <price>63.99</price>\n"
                    + "    </item>\n"
                    + "  </items>\n"
                    + "  <books>\n"
                    + "    <book>\n"
                    + "      <bookId>B005</bookId>\n"
                    + "      <author>Heinlein</author>\n"
                    + "      <review>\n"
                    + "        <reviewId>1</reviewId>\n"
                    + "        <comment>best book ever</comment>\n"
                    + "      </review>\n"
                    + "      <review>\n"
                    + "        <reviewId>2</reviewId>\n"
                    + "        <comment>would recommend</comment>\n"
                    + "      </review>\n"
                    + "    </book>\n"
                    + "    <book>\n"
                    + "      <bookId>B006</bookId>\n"
                    + "      <author>Isaac Asimov</author>\n"
                    + "    </book>\n"
                    + "    <book>\n"
                    + "      <bookId>B008</bookId>\n"
                    + "      <author>Ian M Banks</author>\n"
                    + "    </book>\n"
                    + "  </books>\n"
                    + "</mediaorder>\n";

    /**
     * Pinned per-case step keys rendered as
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|listen. Deploy steps carry the byte-exact EPL
     * text; send payloads render as their compact JSON; listen names the
     * deployed statement the listener attaches to (empty for s11_0).
     */
    private static final Map<String, String[]> CASE_STEPS = new HashMap<>();
    static {
        CASE_STEPS.put("contained-example", new String[]{
                deployKey("contained-example", "s1", EPL_ORD0_S1, "items.item[0].itemId,orderId", "s1"),
                deployKey("contained-example", "s2", EPL_ORD0_S2, "bookId", "s2"),
                deployKey("contained-example", "s3", EPL_ORD0_S3, "bookId", "s3"),
                deployKey("contained-example", "s4", EPL_ORD0_S4, "count(*)", "s4"),
                deployKey("contained-example", "s5", EPL_ORD0_S5, "reviewId", "s5"),
                deployKey("contained-example", "s6", EPL_ORD0_S6, "c.orderId,o.bookId", "s6"),
                deployKey("contained-example", "s7", EPL_ORD0_S7, "bookId,orderId,reviewId", "s7"),
                deployKey("contained-example", "s8", EPL_ORD0_S8, "bookId,reviewId", "s8"),
                deployKey("contained-example", "s9", EPL_ORD0_S9, "bookId,reviewId", "s9"),
                deployKey("contained-example", "s10", EPL_ORD0_S10, "bookId,reviewId", "s10"),
                deployKey("contained-example", "s11_0", EPL_ORD0_S11_0, "", ""),
                deployKey("contained-example", "s11", EPL_ORD0_S11, "book.bookId,mediaOrder.orderId,review.reviewId", "s11"),
                deployKey("contained-example", "s12", EPL_ORD0_S12, "reviewId", "s12"),
                sendXmlKey("contained-example", MEDIA_ORDER_ONE),
                "undeploy-all|contained-example|||||||||||",
        });
        CASE_STEPS.put("solution-pattern", new String[]{
                deployKey("solution-pattern", "s0", EPL_ORD1_S0, "avgTime,category,subEventType", "s0"),
                "send|solution-pattern||SupportResponseEvent||"
                        + "{\"category\":\"svcOne\",\"subEvents\":[{\"responseTimeMillis\":1000,\"subEventType\":\"typeA\"},{\"responseTimeMillis\":800,\"subEventType\":\"typeB\"}]}"
                        + "|||||||",
                "send|solution-pattern||SupportResponseEvent||"
                        + "{\"category\":\"svcOne\",\"subEvents\":[{\"responseTimeMillis\":400,\"subEventType\":\"typeB\"},{\"responseTimeMillis\":500,\"subEventType\":\"typeA\"}]}"
                        + "|||||||",
                "undeploy-all|solution-pattern|||||||||||",
        });
        CASE_STEPS.put("join-self-join", new String[]{
                deployKey("join-self-join", "s0", EPL_JOIN_ROWS_INNER, "book.bookId,item.itemId", "s0"),
                sendXmlKey("join-self-join", MEDIA_ORDER_ONE),
                sendXmlKey("join-self-join", MEDIA_ORDER_ONE),
                sendXmlKey("join-self-join", MEDIA_ORDER_TWO),
                "undeploy-all|join-self-join|||||||||||",
                deployKey("join-self-join", "s0", EPL_JOIN_COUNT_INNER, "count(*)", "s0"),
                sendXmlKey("join-self-join", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-join", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-join|||||||||||",
                deployKey("join-self-join", "s0", EPL_JOIN_UNI_INNER, "count(*)", "s0"),
                sendXmlKey("join-self-join", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-join", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-join|||||||||||",
        });
        CASE_STEPS.put("join-self-left-outer", new String[]{
                deployKey("join-self-left-outer", "s0", EPL_JOIN_ROWS_LEFT, "book.bookId,item.itemId", "s0"),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-left-outer|||||||||||",
                deployKey("join-self-left-outer", "s0", EPL_JOIN_COUNT_LEFT, "count(*)", "s0"),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-left-outer|||||||||||",
                deployKey("join-self-left-outer", "s0", EPL_JOIN_UNI_LEFT, "count(*)", "s0"),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-left-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-left-outer|||||||||||",
        });
        CASE_STEPS.put("join-self-full-outer", new String[]{
                deployKey("join-self-full-outer", "s0", EPL_JOIN_ROWS_FULL, "book.bookId,item.itemId,orderId", "s0"),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-full-outer|||||||||||",
                deployKey("join-self-full-outer", "s0", EPL_JOIN_COUNT_FULL, "count(*)", "s0"),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-full-outer|||||||||||",
                deployKey("join-self-full-outer", "s0", EPL_JOIN_UNI_FULL, "count(*)", "s0"),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_TWO),
                sendXmlKey("join-self-full-outer", MEDIA_ORDER_ONE),
                "undeploy-all|join-self-full-outer|||||||||||",
        });
        CASE_STEPS.put("financial", new String[]{
                deployKey("financial", "module", EPL_FINANCIAL_MODULE,
                        "foreignSymbol,localSymbol,value", "out"),
                fafKey("financial", "ABC", "123"),
                fafKey("financial", "DEF", "456"),
                fafKey("financial", "GHI", "789"),
                fafKey("financial", "JKL", "666"),
                sendMapKey("financial", "ForeignSymbols",
                        "{\"companies\":[{\"symbol\":\"ABC\",\"value\":500.0},{\"symbol\":\"DEF\",\"value\":300.0},{\"symbol\":\"JKL\",\"value\":400.0}]}"),
                sendMapKey("financial", "LocalSymbols",
                        "{\"companies\":[{\"symbol\":\"123\",\"value\":600.0},{\"symbol\":\"456\",\"value\":100.0},{\"symbol\":\"789\",\"value\":200.0}]}"),
                "undeploy-all|financial|||||||||||",
        });
    }

    private static String deployKey(String caseName, String statement, String epl,
                                    String fields, String listen) {
        return "deploy|" + caseName + "|" + statement + "||" + epl + "|||||||"
                + fields + "|" + listen;
    }

    private static String sendXmlKey(String caseName, String xml) {
        JsonObject payload = new JsonObject();
        payload.add("xml", xml);
        return "send|" + caseName + "||MediaOrder||" + payload.toString() + "|||||||";
    }

    private static String sendMapKey(String caseName, String eventType, String payloadJson) {
        return "send|" + caseName + "||" + eventType + "||" + payloadJson + "|||||||";
    }

    private static String fafKey(String caseName, String foreignSymbol, String localSymbol) {
        JsonObject payload = new JsonObject();
        payload.add("foreignSymbol", foreignSymbol);
        payload.add("localSymbol", localSymbol);
        return "faf|" + caseName + "|load-mapping||" + EPL_FAF_MAPPING + "|"
                + payload.toString() + "|||||||";
    }

    private EPLContainedEventExampleScenarioOracle() {
    }

    public static void main(String[] args) throws Exception {
        if (args.length != 1) {
            throw new IllegalArgumentException(
                    "usage: EPLContainedEventExampleScenarioOracle <scenario.json>");
        }
        JsonValue parsed = Json.parse(Files.readString(Path.of(args[0]), StandardCharsets.UTF_8));
        if (!parsed.isObject()) {
            throw new IllegalArgumentException("scenario must be a JSON object");
        }
        rejectDuplicateKeys(parsed);
        JsonObject scenario = parsed.asObject();
        validateScenario(scenario);
        JsonArray allSteps = array(scenario.get("steps"), "steps");

        JsonArray records = new JsonArray();
        for (int caseIndex = 0; caseIndex < CASES.length; caseIndex++) {
            runCase(caseIndex, allSteps, records);
        }
        if (records.size() != EXPECTED_RECORDS) {
            throw new IllegalStateException("expected " + EXPECTED_RECORDS + " records, got "
                    + records.size());
        }

        JsonObject root = new JsonObject();
        root.add("version", VERSION);
        root.add("id", ID);
        root.add("javaCommit", JAVA_COMMIT);
        root.add("java", System.getProperty("java.version"));
        root.add("records", records);
        System.out.println(root.toString());
    }

    /**
     * Replays one case's steps on a fresh runtime (each Java execution gets
     * its own runtime). MediaOrder and Cancel share the XML DOM registration
     * rooted at mediaorder; SupportResponseEvent/SupportResponseSubEvent are
     * preconfigured beans; the internal timer is disabled and the rethrowing
     * exception handler surfaces statement failures to the sender thread.
     * Deploy steps compile their EPL as one module against the runtime path
     * and attach the listener to the step's `listen` statement, mirroring
     * compileDeploy(epl, path).addListener(name).
     */
    private static void runCase(int caseIndex, JsonArray allSteps, JsonArray records)
            throws Exception {
        String caseName = CASES[caseIndex];
        Configuration configuration = new Configuration();
        configuration.getCommon().getEventMeta().setEnableXMLXSD(true);
        ConfigurationCommonEventTypeXMLDOM xmlConfig = new ConfigurationCommonEventTypeXMLDOM();
        xmlConfig.setSchemaResource(mediaOrderSchemaUri());
        xmlConfig.setRootElementName("mediaorder");
        configuration.getCommon().addEventType("MediaOrder", xmlConfig);
        configuration.getCommon().addEventType("Cancel", xmlConfig);
        configuration.getCommon().addEventType(SupportResponseEvent.class);
        configuration.getCommon().addEventType(SupportResponseSubEvent.class);
        configuration.getRuntime().getThreading().setInternalTimerEnabled(false);
        configuration.getRuntime().getExceptionHandling().addClass(
                HarnessRethrowExceptionHandlerFactory.class);
        configuration.getRuntime().getExceptionHandling().setUndeployRethrowPolicy(
                UndeployRethrowPolicy.RETHROW_FIRST);
        EPRuntime runtime = EPRuntimeProvider.getRuntime(ID + "-" + caseName, configuration);
        runtime.getEventService().advanceTime(0);

        Map<String, Integer> sequences = new HashMap<>();
        List<EPCompiled> path = new ArrayList<>();
        EPFireAndForgetPreparedQueryParameterized preparedFAF = null;
        try {
            boolean inCase = false;
            for (JsonValue stepValue : allSteps) {
                JsonObject step = stepValue.asObject();
                String operation = string(step, "op");
                if ("case".equals(operation)) {
                    inCase = caseName.equals(string(step, "case"));
                    continue;
                }
                if (!inCase) {
                    continue;
                }
                switch (operation) {
                    case "deploy": {
                        String epl = string(step, "epl");
                        String listen = string(step, "listen");
                        String[] fields = fieldNames(step.get("fields"));
                        EPDeployment deployment = compileDeploy(runtime, configuration, path, epl);
                        if (!listen.isEmpty()) {
                            for (EPStatement statement : deployment.getStatements()) {
                                if (listen.equals(statement.getName())) {
                                    statement.addListener(
                                            listener(caseName, sequences, records, runtime, fields));
                                }
                            }
                        }
                        break;
                    }
                    case "send":
                        sendEvent(runtime, string(step, "eventType"),
                                object(step.get("payload"), "payload"));
                        break;
                    case "faf": {
                        if (preparedFAF == null) {
                            CompilerArguments compilerArgs =
                                    new CompilerArguments(runtime.getRuntimePath());
                            EPCompiled compiledFAF = EPCompilerProvider.getCompiler()
                                    .compileQuery(string(step, "epl"), compilerArgs);
                            preparedFAF = runtime.getFireAndForgetService()
                                    .prepareQueryWithParameters(compiledFAF);
                        }
                        JsonObject payload = object(step.get("payload"), "payload");
                        preparedFAF.setObject(1, string(payload, "foreignSymbol"));
                        preparedFAF.setObject(2, string(payload, "localSymbol"));
                        runtime.getFireAndForgetService().executeQuery(preparedFAF);
                        break;
                    }
                    case "undeploy-all":
                        runtime.getDeploymentService().undeployAll();
                        break;
                    default:
                        throw new IllegalStateException("unsupported step op " + operation);
                }
            }
        } finally {
            try {
                runtime.getDeploymentService().undeployAll();
            } finally {
                runtime.destroy();
            }
        }
    }

    /** Resolves regression/mediaOrderSchema.xsd under the Esper checkout. */
    private static String mediaOrderSchemaUri() throws Exception {
        String esperRoot = System.getProperty("esperRoot");
        if (esperRoot == null || esperRoot.isEmpty()) {
            throw new IllegalStateException("-DesperRoot is required");
        }
        return Path.of(esperRoot,
                        "regression-run", "etc", "regression", "mediaOrderSchema.xsd")
                .toUri().toURL().toString();
    }

    /** compileDeploy mirrors env.compileDeploy(epl, path): module compile
     * against the full Configuration plus every prior compiled module in the
     * case (matching RegressionPath accumulation) followed by a deployment. */
    private static EPDeployment compileDeploy(EPRuntime runtime, Configuration configuration,
                                              List<EPCompiled> path, String epl)
            throws EPCompileException, EPDeployException {
        CompilerArguments compilerArgs = new CompilerArguments(configuration);
        compilerArgs.getPath().getCompileds().addAll(path);
        EPCompiled compiled = EPCompilerProvider.getCompiler().compile(epl, compilerArgs);
        path.add(compiled);
        return runtime.getDeploymentService().deploy(compiled, new DeploymentOptions());
    }
    /**
     * Listener emitting one record per invocation with a per-statement
     * sequence counter; the default istream selector means only a new
     * array renders and only when non-empty. Rows sort by their compact
     * field rendering because the Java execution asserts the batches with
     * assertPropsPerRowLastNew / assertPropsPerRowNewOnly (order pinned by
     * the EPL order-by where present). Each record projects only the
     * deploy step's pinned fields.
     */
    private static UpdateListener listener(String caseName, Map<String, Integer> sequences,
                                           JsonArray records, EPRuntime runtime,
                                           String[] fields) {
        return (newEvents, oldEvents, statement, ignoredRuntime) -> {
            int sequence = sequences.merge(statement.getName(), 1, Integer::sum);
            JsonObject record = new JsonObject();
            record.add("case", caseName);
            record.add("operation", "listener");
            record.add("statement", statement.getName());
            record.add("sequence", sequence);
            record.add("time",
                    Instant.ofEpochMilli(runtime.getEventService().getCurrentTime()).toString());
            JsonArray newRows = sortedRows(rows(newEvents, fields));
            JsonArray oldRows = sortedRows(rows(oldEvents, fields));
            if (newRows.size() > 0) {
                record.add("new", newRows);
            }
            if (oldRows.size() > 0) {
                record.add("old", oldRows);
            }
            records.add(record);
        };
    }

    /** Canonical row rendering projecting only the pinned field list. */
    private static JsonArray rows(EventBean[] events, String[] fields) {
        JsonArray array = new JsonArray();
        if (events == null) {
            return array;
        }
        for (EventBean event : events) {
            JsonObject item = new JsonObject();
            item.add("kind", "row");
            JsonObject rowFields = new JsonObject();
            for (String name : fields) {
                rowFields.add(name, normalize(event.get(name)));
            }
            item.add("fields", rowFields);
            array.add(item);
        }
        return array;
    }

    /** Sorts rendered rows by their compact field JSON so the any-order
     * Java assertions pin one canonical order on both traces. */
    private static JsonArray sortedRows(JsonArray rows) {
        List<JsonValue> items = new ArrayList<>();
        for (JsonValue item : rows) {
            items.add(item);
        }
        items.sort((left, right) -> left.asObject().get("fields").toString()
                .compareTo(right.asObject().get("fields").toString()));
        JsonArray sorted = new JsonArray();
        for (JsonValue item : items) {
            sorted.add(item);
        }
        return sorted;
    }

    /**
     * Scalar normalization: strings passthrough, integral numbers as JSON
     * numbers, other numbers as doubles, boolean, null as the tagged
     * {"state":"null"} object, and Object[]/int[] row underlyings as JSON
     * arrays.
     */
    private static JsonValue normalize(Object value) {
        if (value == null) {
            JsonObject nullObj = new JsonObject();
            nullObj.add("state", "null");
            return nullObj;
        }
        if (value instanceof Integer || value instanceof Long || value instanceof Short
                || value instanceof Byte) {
            return Json.value(((Number) value).longValue());
        }
        if (value instanceof Number) {
            return Json.value(((Number) value).doubleValue());
        }
        if (value instanceof Boolean) {
            return Json.value((Boolean) value);
        }
        if (value instanceof Object[]) {
            JsonArray items = new JsonArray();
            for (Object element : (Object[]) value) {
                items.add(normalize(element));
            }
            return items;
        }
        if (value instanceof int[]) {
            JsonArray items = new JsonArray();
            for (int element : (int[]) value) {
                items.add(Json.value(element));
            }
            return items;
        }
        return Json.value(String.valueOf(value));
    }

    /**
     * Sends one pinned event: MediaOrder parses the payload's inline XML
     * into a DOM (mirroring SupportXML.getDocument(InputStream), which does
     * not enable namespace awareness); SupportResponseEvent builds the bean
     * from category plus its subEvents array; ForeignSymbols/LocalSymbols
     * send the companies map array via sendEventMap.
     */
    private static void sendEvent(EPRuntime runtime, String type, JsonObject payload) {
        switch (type) {
            case "MediaOrder": {
                runtime.getEventService().sendEventXMLDOM(
                        parseXml(string(payload, "xml")), type);
                return;
            }
            case "SupportResponseEvent": {
                JsonArray subEvents = array(payload.get("subEvents"), "subEvents");
                SupportResponseSubEvent[] subs = new SupportResponseSubEvent[subEvents.size()];
                for (int index = 0; index < subEvents.size(); index++) {
                    JsonObject sub = object(subEvents.get(index), "subEvents element");
                    subs[index] = new SupportResponseSubEvent(
                            longField(sub.get("responseTimeMillis"), "responseTimeMillis"),
                            string(sub, "subEventType"));
                }
                runtime.getEventService().sendEventBean(
                        new SupportResponseEvent(string(payload, "category"), subs), type);
                return;
            }
            case "ForeignSymbols":
            case "LocalSymbols": {
                JsonArray companies = array(payload.get("companies"), "companies");
                Map<String, Object>[] rows = new Map[companies.size()];
                for (int index = 0; index < companies.size(); index++) {
                    JsonObject company = object(companies.get(index), "companies element");
                    Map<String, Object> row = new HashMap<>();
                    row.put("symbol", string(company, "symbol"));
                    row.put("value", doubleField(company.get("value"), "value"));
                    rows[index] = row;
                }
                Map<String, Object> event = new HashMap<>();
                event.put("companies", rows);
                runtime.getEventService().sendEventMap(event, type);
                return;
            }
            default:
                throw new IllegalArgumentException("unknown event type: " + type);
        }
    }

    /** Parses inline XML the way SupportXML.getDocument(InputStream) does:
     * the default DocumentBuilderFactory without namespace awareness. */
    private static Document parseXml(String xml) {
        try {
            DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
            return factory.newDocumentBuilder().parse(new InputSource(new StringReader(xml)));
        } catch (Exception ex) {
            throw new RuntimeException("failed to parse XML payload", ex);
        }
    }

    private static void validateScenario(JsonObject scenario) {
        requireFields(scenario, "version", "id", "description", "javaCommit", "javaSource",
                "javaRuntimes", "javaNames", "javaStaticIds", "javaFlags", "cases", "steps");
        if (!VERSION.equals(string(scenario, "version"))
                || !ID.equals(string(scenario, "id"))
                || !DESCRIPTION.equals(string(scenario, "description"))
                || !JAVA_COMMIT.equals(string(scenario, "javaCommit"))
                || !JAVA_SOURCE.equals(string(scenario, "javaSource"))) {
            throw new IllegalArgumentException("scenario metadata is not pinned");
        }
        validateStringArray(scenario.get("javaRuntimes"), RUNTIME_IDS, "javaRuntimes");
        validateStringArray(scenario.get("javaNames"), EXECUTION_NAMES, "javaNames");
        validateStringArray(scenario.get("javaStaticIds"), STATIC_IDS, "javaStaticIds");
        validateStringArray(scenario.get("javaFlags"), JAVA_FLAGS, "javaFlags");

        JsonArray cases = array(scenario.get("cases"), "cases");
        if (cases.size() != CASES.length) {
            throw new IllegalArgumentException("scenario must contain exactly "
                    + CASES.length + " cases");
        }
        for (int index = 0; index < cases.size(); index++) {
            JsonObject definition = object(cases.get(index), "case definition");
            requireFields(definition, "case", "ordinal", "runtimeId", "executionName",
                    "observation", "epl");
            if (!CASES[index].equals(string(definition, "case"))
                    || integer(definition, "ordinal") != ORDINALS[index]
                    || !RUNTIME_IDS[index].equals(string(definition, "runtimeId"))
                    || !EXECUTION_NAMES[index].equals(string(definition, "executionName"))
                    || !CASE_OBSERVATIONS[index].equals(string(definition, "observation"))
                    || !caseEpl(CASES[index]).equals(string(definition, "epl"))) {
                throw new IllegalArgumentException("case metadata is not pinned at index " + index);
            }
        }

        JsonArray steps = array(scenario.get("steps"), "steps");
        if (steps.size() != EXPECTED_STEPS) {
            throw new IllegalArgumentException("scenario must contain exactly " + EXPECTED_STEPS
                    + " steps, got " + steps.size());
        }
        int offset = 0;
        for (String caseName : CASES) {
            validateCaseMarker(steps.get(offset++), caseName);
            String[] expected = CASE_STEPS.get(caseName);
            for (String key : expected) {
                JsonObject step = object(steps.get(offset++), "step");
                String actual = stepKey(step);
                if (!key.equals(actual)) {
                    throw new IllegalArgumentException("step is not pinned for " + caseName
                            + ": expected [" + key + "] got [" + actual + "]");
                }
            }
        }
        if (offset != steps.size()) {
            throw new IllegalArgumentException("scenario steps contain an unexpected suffix");
        }
    }

    /** The pinned cases[] epl: the newline-joined EPL of every deploy step
     * in the case, in step order. */
    private static String caseEpl(String caseName) {
        String[] keys = CASE_STEPS.get(caseName);
        StringBuilder text = new StringBuilder();
        for (String key : keys) {
            if (!key.startsWith("deploy|")) {
                continue;
            }
            String[] segments = key.split("\\|", -1);
            if (text.length() > 0) {
                text.append('\n');
            }
            text.append(segments[4]);
        }
        return text.toString();
    }

    private static void validateCaseMarker(JsonValue value, String expectedCase) {
        JsonObject marker = object(value, "case marker");
        requireFields(marker, "op", "case");
        if (!"case".equals(string(marker, "op")) || !expectedCase.equals(string(marker, "case"))) {
            throw new IllegalArgumentException("case marker is not pinned for " + expectedCase);
        }
    }

    /**
     * Renders one step as its pinned key:
     * op|case|statement|eventType|epl|payload|expectError|compileWithoutPath|
     * mode|selector|ids|fields|listen with the payload compacted and
     * ids/fields rendered as JSON arrays. Unknown fields are rejected.
     */
    private static String stepKey(JsonObject step) {
        Set<String> allowed = new HashSet<>(Arrays.asList(
                "op", "case", "statement", "eventType", "epl", "payload",
                "expectError", "compileWithoutPath", "mode", "selector", "ids", "fields",
                "listen"));
        for (String field : step.names()) {
            if (!allowed.contains(field)) {
                throw new IllegalArgumentException("step has unexpected field " + field);
            }
        }
        JsonValue payload = step.get("payload");
        String payloadText = payload == null ? "" : payload.toString();
        String cwp = step.getBoolean("compileWithoutPath", false) ? "1" : "";
        JsonValue ids = step.get("ids");
        String idsText = ids == null ? "" : ids.toString();
        JsonValue fields = step.get("fields");
        String fieldsText = fields == null ? "" : joinStrings(fields);
        return string(step, "op") + "|" + string(step, "case") + "|" + string(step, "statement")
                + "|" + string(step, "eventType") + "|" + string(step, "epl") + "|" + payloadText
                + "|" + string(step, "expectError") + "|" + cwp
                + "|" + string(step, "mode") + "|" + string(step, "selector") + "|" + idsText
                + "|" + fieldsText + "|" + string(step, "listen");
    }

    private static String[] fieldNames(JsonValue value) {
        if (value == null) {
            return new String[0];
        }
        JsonArray items = array(value, "fields");
        String[] names = new String[items.size()];
        for (int index = 0; index < items.size(); index++) {
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            names[index] = item.asString();
        }
        return names;
    }

    private static String joinStrings(JsonValue value) {
        JsonArray items = array(value, "fields");
        StringBuilder text = new StringBuilder();
        for (int index = 0; index < items.size(); index++) {
            if (index > 0) {
                text.append(',');
            }
            JsonValue item = items.get(index);
            if (!(item instanceof JsonString)) {
                throw new IllegalArgumentException("fields must be a string array");
            }
            text.append(item.asString());
        }
        return text.toString();
    }

    private static void rejectDuplicateKeys(JsonValue value) {
        if (value.isObject()) {
            Set<String> names = new HashSet<>();
            for (Member member : value.asObject()) {
                if (!names.add(member.getName())) {
                    throw new IllegalArgumentException("duplicate JSON object key: " + member.getName());
                }
                rejectDuplicateKeys(member.getValue());
            }
        } else if (value.isArray()) {
            for (JsonValue item : value.asArray()) {
                rejectDuplicateKeys(item);
            }
        }
    }

    private static void requireFields(JsonObject object, String... expectedNames) {
        if (object == null || object.size() != expectedNames.length
                || !new HashSet<>(object.names()).equals(new HashSet<>(Arrays.asList(expectedNames)))) {
            throw new IllegalArgumentException("JSON object has unexpected fields");
        }
    }

    private static JsonObject object(JsonValue value, String label) {
        if (value == null || !value.isObject()) {
            throw new IllegalArgumentException(label + " must be a JSON object");
        }
        return value.asObject();
    }

    private static JsonArray array(JsonValue value, String label) {
        if (value == null || !value.isArray()) {
            throw new IllegalArgumentException(label + " must be a JSON array");
        }
        return value.asArray();
    }

    private static String string(JsonObject object, String name) {
        JsonValue value = object.get(name);
        if (value == null) {
            return "";
        }
        if (!(value instanceof JsonString)) {
            throw new IllegalArgumentException(name + " must be a JSON string");
        }
        return value.asString();
    }

    private static int integer(JsonObject object, String name) {
        long value = longInteger(object.get(name), name);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new IllegalArgumentException(name + " is outside the Java int range");
        }
        return (int) value;
    }

    private static long longField(JsonValue value, String label) {
        return longInteger(value, label);
    }

    private static double doubleField(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON number");
        }
        return Double.parseDouble(value.toString());
    }

    private static long longInteger(JsonValue value, String label) {
        if (!(value instanceof JsonNumber)) {
            throw new IllegalArgumentException(label + " must be a JSON integer");
        }
        String text = value.toString();
        try {
            return Long.parseLong(text, 10);
        } catch (NumberFormatException ex) {
            throw new IllegalArgumentException(label + " is outside the Java long range", ex);
        }
    }

    private static void validateStringArray(JsonValue value, String[] expected, String label) {
        JsonArray actual = array(value, label);
        if (actual.size() != expected.length) {
            throw new IllegalArgumentException(label + " length is not pinned");
        }
        for (int index = 0; index < expected.length; index++) {
            JsonValue item = actual.get(index);
            if (!(item instanceof JsonString) || !expected[index].equals(item.asString())) {
                throw new IllegalArgumentException(label + " mismatch at index " + index);
            }
        }
    }

    /**
     * Bean mirroring regression-lib SupportResponseEvent: a category plus a
     * SupportResponseSubEvent array; Esper expands [subEvents] over the
     * array property.
     */
    public static class SupportResponseEvent {
        private final String category;
        private final SupportResponseSubEvent[] subEvents;

        public SupportResponseEvent(String category, SupportResponseSubEvent[] subEvents) {
            this.category = category;
            this.subEvents = subEvents;
        }

        public String getCategory() {
            return category;
        }

        public SupportResponseSubEvent[] getSubEvents() {
            return subEvents;
        }
    }

    /**
     * Bean mirroring regression-lib SupportResponseSubEvent: a
     * responseTimeMillis long plus a subEventType string.
     */
    public static class SupportResponseSubEvent {
        private final long responseTimeMillis;
        private final String subEventType;

        public SupportResponseSubEvent(long responseTimeMillis, String subEventType) {
            this.responseTimeMillis = responseTimeMillis;
            this.subEventType = subEventType;
        }

        public long getResponseTimeMillis() {
            return responseTimeMillis;
        }

        public String getSubEventType() {
            return subEventType;
        }
    }

    /** Mirrors SupportExceptionHandlerFactoryRethrow from the regression harness. */
    public static class HarnessRethrowExceptionHandlerFactory implements ExceptionHandlerFactory {
        @Override
        public ExceptionHandler getHandler(ExceptionHandlerFactoryContext context) {
            return handlerContext -> {
                throw new RuntimeException("Unexpected exception in statement '"
                        + handlerContext.getStatementName() + "': "
                        + handlerContext.getThrowable().getMessage(),
                        handlerContext.getThrowable());
            };
        }
    }
}
