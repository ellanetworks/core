// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"testing"
)

// Algorithm test data transcribed from 3GPP TS 33.401 Annex C, "Algorithm test
// data", each vector named by the clause it comes from. The same values verify
// both generations: TS 33.501 Annex D defines 128-NEA1/128-NIA1 and
// 128-NEA2/128-NIA2 as 128-EEA1/128-EIA1 and 128-EEA2/128-EIA2 unchanged, and
// this package implements each algorithm once for both.
//
// Annex C states lengths in bits and writes the payload left-aligned, padded with
// zero bits to a word boundary. The integrity sets below are the byte-aligned ones
// the annex names for EPS verification (C.2 sets 2, 5 and 8; C.4 sets 1, 4 and 7),
// since NAS messages are byte aligned and this package's MAC takes whole octets.
// The ciphering sets need no such restriction: a keystream XOR is defined per bit,
// so a vector of any length is checked over the bits the annex declares.

// cipherVector is one 128-EEA2 / 128-NEA2 test set.
type cipherVector struct {
	set    string
	key    string
	count  uint32
	bearer Bearer
	dir    Direction
	bits   int
	plain  string
	cipher string
}

// integrityVector is one 128-EIA1 / 128-EIA2 test set (and their 5GS twins).
type integrityVector struct {
	set    string
	key    string
	count  uint32
	bearer Bearer
	dir    Direction
	bits   int
	msg    string
	mac    string
}

var eea2Vectors = []cipherVector{
	{
		set:    "C.1.1",
		key:    "d3c5d592327fb11c4035c6680af8c6d1",
		count:  0x398a59b4,
		bearer: 0x15,
		dir:    DirectionDownlink,
		bits:   253,
		plain: "981ba6824c1bfb1ab485472029b71d80" +
			"8ce33e2cc3c0b5fc1f3de8a6dc66b1f0",
		cipher: "e9fed8a63d155304d71df20bf3e82214" +
			"b20ed7dad2f233dc3c22d7bdeeed8e78",
	},
	{
		set:    "C.1.2",
		key:    "2bd6459f82c440e0952c49104805ff48",
		count:  0xc675a64b,
		bearer: 0x0c,
		dir:    DirectionDownlink,
		bits:   798,
		plain: "7ec61272743bf1614726446a6c38ced1" +
			"66f6ca76eb5430044286346cef130f92" +
			"922b03450d3a9975e5bd2ea0eb55ad8e" +
			"1b199e3ec4316020e9a1b285e7627953" +
			"59b7bdfd39bef4b2484583d5afe082ae" +
			"e638bf5fd5a606193901a08f4ab41aab" +
			"9b134880",
		cipher: "5961605353c64bdca15b195e288553a9" +
			"10632506d6200aa790c4c806c99904cf" +
			"2445cc50bb1cf168a49673734e081b57" +
			"e324ce5259c0e78d4cd97b870976503c" +
			"0943f2cb5ae8f052c7b7d392239587b8" +
			"956086bcab18836042e2e6ce42432a17" +
			"105c53d0",
	},
	{
		set:    "C.1.3",
		key:    "0a8b6bd8d9b08b08d64e32d1817777fb",
		count:  0x544d49cd,
		bearer: 0x04,
		dir:    DirectionUplink,
		bits:   310,
		plain: "fd40a41d370a1f65745095687d47ba1d" +
			"36d2349e23f644392c8ea9c49d40c132" +
			"71aff264d0f24800",
		cipher: "75750d37b4bba2a4dedb34235bd68c66" +
			"45acdaaca48138a3b0c471e2a7041a57" +
			"6423d2927287f000",
	},
	{
		set:    "C.1.4",
		key:    "aa1f95aea533bcb32eb63bf52d8f831a",
		count:  0x72d8c671,
		bearer: 0x10,
		dir:    DirectionDownlink,
		bits:   1022,
		plain: "fb1b96c5c8badfb2e8e8edfde78e57f2" +
			"ad81e74103fc430a534dcc37afcec70e" +
			"1517bb06f27219dae49022ddc47a068d" +
			"e4c9496a951a6b09edbdc864c7adbd74" +
			"0ac50c022f3082bafd22d78197c5d508" +
			"b977bca13f32e652e74ba728576077ce" +
			"628c535e87dc6077ba07d29068590c8c" +
			"b5f1088e082cfa0ec961302d69cf3d44",
		cipher: "dfb440acb3773549efc04628aeb8d815" +
			"6275230bdc690d94b00d8d95f28c4b56" +
			"307f60f4ca55eba661ebba72ac808fa8" +
			"c49e26788ed04a5d606cb418de74878b" +
			"9a22f8ef29590bc4eb57c9faf7c41524" +
			"a885b8979c423f2f8f8e0592a9879201" +
			"be7ff9777a162ab810feb324ba74c4c1" +
			"56e04d39097209653ac33e5a5f2d8864",
	},
	{
		set:    "C.1.5",
		key:    "9618ae46891f86578eebe90ef7a1202e",
		count:  0xc675a64b,
		bearer: 0x0c,
		dir:    DirectionDownlink,
		bits:   1245,
		plain: "8daa17b1ae050529c6827f28c0ef6a12" +
			"42e93f8b314fb18a77f790ae049fedd6" +
			"12267fecaefc450174d76d9f9aa7755a" +
			"30cd90a9a5874bf48eaf70eea3a62a25" +
			"0a8b6bd8d9b08b08d64e32d1817777fb" +
			"544d49cd49720e219dbf8bbed33904e1" +
			"fd40a41d370a1f65745095687d47ba1d" +
			"36d2349e23f644392c8ea9c49d40c132" +
			"71aff264d0f24841d6465f0996ff84e6" +
			"5fc517c53efc3363c38492a8",
		cipher: "919c8c33d66789703d05a0d7ce82a2ae" +
			"ac4ee76c0f4da050335e8a84e7897ba5" +
			"df2f36bd513e3d0c8578c7a0fcf043e0" +
			"3aa3a39fbaad7d15be074faa5d9029f7" +
			"1fb457b647834714b0e18f117fca1067" +
			"7945096c8c5f326ba8d6095eb29c3e36" +
			"cf245d1622aafe921f7566c4f5d644f2" +
			"f1fc0ec684ddb21349747622e209295d" +
			"27ff3f95623371d49b147c0af486171f" +
			"22cd04b1cbeb2658223e6938",
	},
	{
		set:    "C.1.6",
		key:    "54f4e2e04c83786eec8fb5abe8e36566",
		count:  0xaca4f50f,
		bearer: 0x0b,
		dir:    DirectionUplink,
		bits:   3861,
		plain: "40981ba6824c1bfb4286b299783daf44" +
			"2c099f7ab0f58d5c8e46b104f08f01b4" +
			"1ab485472029b71d36bd1a3d90dc3a41" +
			"b46d51672ac4c9663a2be063da4bc8d2" +
			"808ce33e2cccbfc634e1b259060876a0" +
			"fbb5a437ebcc8d31c19e4454318745e3" +
			"fa16bb11adae248879fe52db2543e53c" +
			"f445d3d828ce0bf5c560593d97278a59" +
			"762dd0c2c9cd68d4496a792508614014" +
			"b13b6aa51128c18cd6a90b87978c2ff1" +
			"cabe7d9f898a411bfdb84f68f6727b14" +
			"99cdd30df0443ab4a66653330bcba110" +
			"5e4cec034c73e605b4310eaaadcfd5b0" +
			"ca27ffd89d144df4792759427c9cc1f8" +
			"cd8c87202364b8a687954cb05a8d4e2d" +
			"99e73db160deb180ad0841e96741a5d5" +
			"9fe4189f15420026fe4cd12104932fb3" +
			"8f735340438aaf7eca6fd5cfd3a195ce" +
			"5abe65272af607ada1be65a6b4c9c069" +
			"3234092c4d018f1756c6db9dc8a6d80b" +
			"888138616b681262f954d0e771174878" +
			"0d92291d86299972db741cfa4f37b8b5" +
			"6cdb18a7ca8218e86e4b4b716a4d0437" +
			"1fbec262fc5ad0b3819b187b97e55b1a" +
			"4d7c19ee24c8b4d7723cfedf045b8aca" +
			"e4869517d80e50615d9035d5d9c5a40a" +
			"f602280b542597b0cb18619eeb359257" +
			"59d195e100e8e4aa0c38a3c2abe0f3d8" +
			"ff04f3c33c295069c23694b5bbeacdd5" +
			"42e28e8a94edb9119f412d054be1fa72" +
			"00b09000",
		cipher: "5cb72c6edc878f1566e10253afc364c9" +
			"fa540d914db94cbee275d0917ca6af0d" +
			"77acb4ef3bbe1a722b2ef5bd1d4b8e2a" +
			"a5024ec1388a201e7bce7920aec61589" +
			"5f763a5564dcc4c482a2ee1d8bfecc44" +
			"98eca83fbb75f9ab530e0dafbede2fa5" +
			"895b82991b6277c529e0f2529d7f7960" +
			"6be96706296dedfa9d7412b616958cb5" +
			"63c678c02825c30d0aee77c4c146d276" +
			"5412421a808d13cec819694c75ad572e" +
			"9b973d948b81a9337c3b2a17192e22c2" +
			"069f7ed1162af44cdea817603665e807" +
			"ce40c8e0dd9d6394dc6e31153fe1955c" +
			"47afb51f2617ee0c5e3b8ef1ad7574ed" +
			"343edc2743cc94c990e1f1fd264253c1" +
			"78dea739c0befeebcd9f9b76d49c1015" +
			"c9fecf50e53b8b5204dbcd3eed863855" +
			"dabcdcc94b31e318021568855c8b9e52" +
			"a981957a112827f978ba960f1447911b" +
			"317b5511fbcc7fb13ac153db74251117" +
			"e4861eb9e83bffffc4eb7755579038e5" +
			"7924b1f78b3e1ad90bab2a07871b72db" +
			"5eef96c334044966db0c37cafd1a89e5" +
			"646a3580eb6465f121dce9cb88d85b96" +
			"cf23ccccd4280767bee8eeb23d865246" +
			"1db6493103003baf89f5e18261ea43c8" +
			"4a92ebffffe4909dc46c5192f825f770" +
			"600b9602c557b5f8b431a79d45977dd9" +
			"c41b863da9e142e90020cfd074d6927b" +
			"7ab3b6725d1a6f3f98b9c9daa8982aff" +
			"06782800",
	},
}

var eia2Vectors = []integrityVector{
	{
		set:    "C.2.2",
		key:    "d3c5d592327fb11c4035c6680af8c6d1",
		count:  0x398a59b4,
		bearer: 0x1a,
		dir:    DirectionDownlink,
		bits:   64,
		msg:    "484583d5afe082ae",
		mac:    "b93787e6",
	},
	{
		set:    "C.2.5",
		key:    "83fd23a244a74cf358da3019f1722635",
		count:  0x36af6144,
		bearer: 0x0f,
		dir:    DirectionDownlink,
		bits:   768,
		msg: "35c68716633c66fb750c266865d53c11" +
			"ea05b1e9fa49c8398d48e1efa5909d39" +
			"47902837f5ae96d5a05bc8d61ca8dbef" +
			"1b13a4b4abfe4fb1006045b674bb5472" +
			"9304c382be53a5af05556176f6eaa2ef" +
			"1d05e4b083181ee674cda5a485f74d7a",
		mac: "e657e182",
	},
	{
		set:    "C.2.8",
		key:    "b3120ffdb2cf6af4e73eaf2ef4ebec69",
		count:  0x296f393c,
		bearer: 0x0b,
		dir:    DirectionDownlink,
		bits:   16448,
		msg: "00000000000000000101010101010101" +
			"e0958045f3a0bba4e3968346f0a3b8a7" +
			"c02a018ae640765226b987c913e6cbf0" +
			"83570016cf83efbc61c082513e21561a" +
			"427c009d28c298eface78ed6d56c2d45" +
			"05ad032e9c04dc60e73a81696da665c6" +
			"c48603a57b45ab33221585e68ee31691" +
			"87fb0239528632dd656c807ea3248b7b" +
			"46d002b2b5c7458eb85b9ce95879e034" +
			"0859055e3b0abbc3eace8719caa80265" +
			"c97205d5dc4bcc902fe1839629ed7132" +
			"8a0f0449f588557e6898860e042aecd8" +
			"4b2404c212c9222da5bf8a89ef679787" +
			"0cf50771a60f66a2ee62853657addf04" +
			"cdde07fa414e11f12b4d81b9b4e8ac53" +
			"8ea30666688d881f6c348421992f31b9" +
			"4f8806ed8fccff4c9123b89642527ad6" +
			"13b109bf75167485f1268bf884b4cd23" +
			"d29a0934925703d634098f7767f1be74" +
			"91e708a8bb949a3873708aef4a36239e" +
			"50cc08235cd5ed6bbe578668a17b58c1" +
			"171d0b90e813a9e4f58a89d719b11042" +
			"d6360b1b0f52deb730a58d58faf46315" +
			"954b0a872691475977dc88c0d733feff" +
			"54600a0cc1d0300aaaeb94572c6e95b0" +
			"1ae90de04f1dce47f87e8fa7bebf77e1" +
			"dbc20d6ba85cb9143d518b285dfa04b6" +
			"98bf0cf7819f20fa7a288eb0703d995c" +
			"59940c7c66de57a9b70f82379b70e203" +
			"1e450fcfd2181326fcd28d8823baaa80" +
			"df6e0f443559647539fd8907c0ffd9d7" +
			"9c130ed81c9afd9b7e848c9fed38443d" +
			"5d380e53fbdb8ac8c3d3f06876054f12" +
			"2461107de92fea09c6f6923a188d53af" +
			"e54a10f60e6e9d5a03d996b5fbc820f8" +
			"a637116a27ad04b444a0932dd60fbd12" +
			"671c11e1c0ec73e789879faa3d42c64d" +
			"20cd1252742a3768c25a901585888ece" +
			"e1e612d9936b403b0775949a66cdfd99" +
			"a29b1345baa8d9d5400c91024b0a6073" +
			"63b013ce5de9ae869d3b8d95b0570b3c" +
			"2d391422d32450cbcfae96652286e96d" +
			"ec1214a9346527980a8192eac1c39a3a" +
			"af6f15351da6be764df89772ec0407d0" +
			"6e4415befae7c92580df9bf507497c8f" +
			"2995160d4e218daacb02944abf83340c" +
			"e8be1686a960faf90e2d90c55cc6475b" +
			"abc3171a80a363174954955d7101dab1" +
			"6ae8179167e21444b443a9eaaa7c91de" +
			"36d118c39d389f8dd4469a846c9a262b" +
			"f7fa18487a79e8de11699e0b8fdf557c" +
			"b48719d453ba713056109b93a218c896" +
			"75ac195fb4fb06639b3797144955b3c9" +
			"327d1aec003d42ecd0ea98abf19ffb4a" +
			"f3561a67e77c35bf15c59c2412da881d" +
			"b02b1bfbcebfac5152bc99bc3f1d15f7" +
			"71001b7029fedb028f8b852bc4407eb8" +
			"3f891c9ca733254fdd1e9edb56919ce9" +
			"fea21c174072521c18319a54b5d4efbe" +
			"bddf1d8b69b1cbf25f489fcc98137254" +
			"7cf41d008ef0bca1926f934b735e090b" +
			"3b251eb33a36f82ed9b29cf4cb944188" +
			"fa0e1e38dd778f7d1c9d987b28d132df" +
			"b9731fa4f4b416935be49de30516af35" +
			"78581f2f13f561c0663361941eab249a" +
			"4bc123f8d15cd711a956a1bf20fe6eb7" +
			"8aea2373361da0426c79a530c3bb1de0" +
			"c99722ef1fde39ac2b00a0a8ee7c800a" +
			"08bc2264f89f4effe627ac2f0531fb55" +
			"4f6d21d74c590a70adfaa390bdfbb3d6" +
			"8e46215cab187d2368d5a71f5ebec081" +
			"cd3b20c082dbe4cd2faca28773795d6b" +
			"0c10204b659a939ef29bbe1088243624" +
			"429927a7eb576dd3a00ea5e01af5d475" +
			"83b2272c0c161a806521a16ff9b0a722" +
			"c0cf26b025d5836e2258a4f7d4773ac8" +
			"01e4263bc294f43def7fa8703f3a4197" +
			"463525887652b0b2a4a2a7cf87f00914" +
			"871e25039113c7e1618da34064b57a43" +
			"c463249fb8d05e0f26f4a6d84972e7a9" +
			"054824145f91295cdbe39a6f920facc6" +
			"59712b46a54ba295bbe6a90154e91b33" +
			"985a2bcd420ad5c67ec9ad8eb7ac6864" +
			"db272a516bc94c2839b0a8169a6bf58e" +
			"1a0c2ada8c883b7bf497a49171268ed1" +
			"5ddd2969384e7ff4bf4aab2ec9ecc652" +
			"9cf629e2df0f08a77a65afa12aa9b505" +
			"df8b287ef6cc91493d1caa39076e28ef" +
			"1ea028f5118de61ae02bb6aefc3343a0" +
			"50292f199f401857b2bead5e6ee2a1f1" +
			"91022f9278016f047791a9d18da7d2a6" +
			"d27f2e0e51c2f6ea30e8ac49a0604f4c" +
			"13542e85b68381b9fdcfa0ce4b2d3413" +
			"54852d360245c536b612af71f3e77c90" +
			"95ae2dbde504b265733dabfe10a20fc7" +
			"d6d32c21ccc72b8b3444ae663d65922d" +
			"17f82caa2b865cd88913d291a6589902" +
			"6ea1328439723c198c36b0c3c8d085bf" +
			"af8a320fde334b4a4919b44c2b95f6e8" +
			"ecf73393f7f0d2a40e60b1d406526b02" +
			"2ddc331810b1a5f7c347bd53ed1f105d" +
			"6a0d30aba477e178889ab2ec55d558de" +
			"ab2630204336962b4db5b663b6902b89" +
			"e85b31bc6af50fc50accb3fb9b57b663" +
			"297031378db47896d7fbaf6c600add2c" +
			"67f936db037986db856eb49cf2db3f7d" +
			"a6d23650e438f1884041b013119e4c2a" +
			"e5af37cccdfb68660738b58b3c59d1c0" +
			"248437472aba1f35ca1fb90cd714aa9f" +
			"635534f49e7c5bba81c2b6b36fdee21c" +
			"a27e347f793d2ce944edb23c8c9b914b" +
			"e10335e350feb5070394b7a4a15c0ca1" +
			"20283568b7bfc254fe838b137a2147ce" +
			"7c113a3a4d65499d9e86b87dbcc7f03b" +
			"bd3a3ab1aa243ece5ba9bcf25f82836c" +
			"fe473b2d83e7a7201cd0b96a72451e86" +
			"3f6c3ba664a6d073d1f7b5ed990865d9" +
			"78bd3815d06094fc9a2aba5221c22d5a" +
			"b996389e3721e3af5f05beddc2875e0d" +
			"faeb39021ee27a41187cbb45ef40c3e7" +
			"3bc03989f9a30d12c54ba7d2141da8a8" +
			"75493e65776ef35f97debc2286cc4af9" +
			"b4623eee902f840c52f1b8ad658939ae" +
			"f71f3f72b9ec1de21588bd35484ea444" +
			"36343ff95ead6ab1d8afb1b2a303df1b" +
			"71e53c4aea6b2e3e9372be0d1bc99798" +
			"b0ce3cc10d2a596d565dba82f88ce4cf" +
			"f3b33d5d24e9c0831124bf1ad54b7925" +
			"32983dd6c3a8b7d0",
		mac: "ebd5ccb0",
	},
}

var eia1Vectors = []integrityVector{
	{
		set:    "C.4.1",
		key:    "2bd6459f82c5b300952c49104881ff48",
		count:  0x38a6f056,
		bearer: 0x1f,
		dir:    DirectionUplink,
		bits:   88,
		msg:    "33323462633938613734790000000000",
		mac:    "731f1165",
	},
	{
		set:    "C.4.4",
		key:    "83fd23a244a74cf358da3019f1722635",
		count:  0x36af6144,
		bearer: 0x0f,
		dir:    DirectionDownlink,
		bits:   768,
		msg: "35c68716633c66fb750c266865d53c11" +
			"ea05b1e9fa49c8398d48e1efa5909d39" +
			"47902837f5ae96d5a05bc8d61ca8dbef" +
			"1b13a4b4abfe4fb1006045b674bb5472" +
			"9304c382be53a5af05556176f6eaa2ef" +
			"1d05e4b083181ee674cda5a485f74d7a",
		mac: "bba74492",
	},
	{
		set:    "C.4.7",
		key:    "b3120ffdb2cf6af4e73eaf2ef4ebec69",
		count:  0x296f393c,
		bearer: 0x0b,
		dir:    DirectionDownlink,
		bits:   16448,
		msg: "00000000000000000101010101010101" +
			"e0958045f3a0bba4e3968346f0a3b8a7" +
			"c02a018ae640765226b987c913e6cbf0" +
			"83570016cf83efbc61c082513e21561a" +
			"427c009d28c298eface78ed6d56c2d45" +
			"05ad032e9c04dc60e73a81696da665c6" +
			"c48603a57b45ab33221585e68ee31691" +
			"87fb0239528632dd656c807ea3248b7b" +
			"46d002b2b5c7458eb85b9ce95879e034" +
			"0859055e3b0abbc3eace8719caa80265" +
			"c97205d5dc4bcc902fe1839629ed7132" +
			"8a0f0449f588557e6898860e042aecd8" +
			"4b2404c212c9222da5bf8a89ef679787" +
			"0cf50771a60f66a2ee62853657addf04" +
			"cdde07fa414e11f12b4d81b9b4e8ac53" +
			"8ea30666688d881f6c348421992f31b9" +
			"4f8806ed8fccff4c9123b89642527ad6" +
			"13b109bf75167485f1268bf884b4cd23" +
			"d29a0934925703d634098f7767f1be74" +
			"91e708a8bb949a3873708aef4a36239e" +
			"50cc08235cd5ed6bbe578668a17b58c1" +
			"171d0b90e813a9e4f58a89d719b11042" +
			"d6360b1b0f52deb730a58d58faf46315" +
			"954b0a872691475977dc88c0d733feff" +
			"54600a0cc1d0300aaaeb94572c6e95b0" +
			"1ae90de04f1dce47f87e8fa7bebf77e1" +
			"dbc20d6ba85cb9143d518b285dfa04b6" +
			"98bf0cf7819f20fa7a288eb0703d995c" +
			"59940c7c66de57a9b70f82379b70e203" +
			"1e450fcfd2181326fcd28d8823baaa80" +
			"df6e0f443559647539fd8907c0ffd9d7" +
			"9c130ed81c9afd9b7e848c9fed38443d" +
			"5d380e53fbdb8ac8c3d3f06876054f12" +
			"2461107de92fea09c6f6923a188d53af" +
			"e54a10f60e6e9d5a03d996b5fbc820f8" +
			"a637116a27ad04b444a0932dd60fbd12" +
			"671c11e1c0ec73e789879faa3d42c64d" +
			"20cd1252742a3768c25a901585888ece" +
			"e1e612d9936b403b0775949a66cdfd99" +
			"a29b1345baa8d9d5400c91024b0a6073" +
			"63b013ce5de9ae869d3b8d95b0570b3c" +
			"2d391422d32450cbcfae96652286e96d" +
			"ec1214a9346527980a8192eac1c39a3a" +
			"af6f15351da6be764df89772ec0407d0" +
			"6e4415befae7c92580df9bf507497c8f" +
			"2995160d4e218daacb02944abf83340c" +
			"e8be1686a960faf90e2d90c55cc6475b" +
			"abc3171a80a363174954955d7101dab1" +
			"6ae8179167e21444b443a9eaaa7c91de" +
			"36d118c39d389f8dd4469a846c9a262b" +
			"f7fa18487a79e8de11699e0b8fdf557c" +
			"b48719d453ba713056109b93a218c896" +
			"75ac195fb4fb06639b3797144955b3c9" +
			"327d1aec003d42ecd0ea98abf19ffb4a" +
			"f3561a67e77c35bf15c59c2412da881d" +
			"b02b1bfbcebfac5152bc99bc3f1d15f7" +
			"71001b7029fedb028f8b852bc4407eb8" +
			"3f891c9ca733254fdd1e9edb56919ce9" +
			"fea21c174072521c18319a54b5d4efbe" +
			"bddf1d8b69b1cbf25f489fcc98137254" +
			"7cf41d008ef0bca1926f934b735e090b" +
			"3b251eb33a36f82ed9b29cf4cb944188" +
			"fa0e1e38dd778f7d1c9d987b28d132df" +
			"b9731fa4f4b416935be49de30516af35" +
			"78581f2f13f561c0663361941eab249a" +
			"4bc123f8d15cd711a956a1bf20fe6eb7" +
			"8aea2373361da0426c79a530c3bb1de0" +
			"c99722ef1fde39ac2b00a0a8ee7c800a" +
			"08bc2264f89f4effe627ac2f0531fb55" +
			"4f6d21d74c590a70adfaa390bdfbb3d6" +
			"8e46215cab187d2368d5a71f5ebec081" +
			"cd3b20c082dbe4cd2faca28773795d6b" +
			"0c10204b659a939ef29bbe1088243624" +
			"429927a7eb576dd3a00ea5e01af5d475" +
			"83b2272c0c161a806521a16ff9b0a722" +
			"c0cf26b025d5836e2258a4f7d4773ac8" +
			"01e4263bc294f43def7fa8703f3a4197" +
			"463525887652b0b2a4a2a7cf87f00914" +
			"871e25039113c7e1618da34064b57a43" +
			"c463249fb8d05e0f26f4a6d84972e7a9" +
			"054824145f91295cdbe39a6f920facc6" +
			"59712b46a54ba295bbe6a90154e91b33" +
			"985a2bcd420ad5c67ec9ad8eb7ac6864" +
			"db272a516bc94c2839b0a8169a6bf58e" +
			"1a0c2ada8c883b7bf497a49171268ed1" +
			"5ddd2969384e7ff4bf4aab2ec9ecc652" +
			"9cf629e2df0f08a77a65afa12aa9b505" +
			"df8b287ef6cc91493d1caa39076e28ef" +
			"1ea028f5118de61ae02bb6aefc3343a0" +
			"50292f199f401857b2bead5e6ee2a1f1" +
			"91022f9278016f047791a9d18da7d2a6" +
			"d27f2e0e51c2f6ea30e8ac49a0604f4c" +
			"13542e85b68381b9fdcfa0ce4b2d3413" +
			"54852d360245c536b612af71f3e77c90" +
			"95ae2dbde504b265733dabfe10a20fc7" +
			"d6d32c21ccc72b8b3444ae663d65922d" +
			"17f82caa2b865cd88913d291a6589902" +
			"6ea1328439723c198c36b0c3c8d085bf" +
			"af8a320fde334b4a4919b44c2b95f6e8" +
			"ecf73393f7f0d2a40e60b1d406526b02" +
			"2ddc331810b1a5f7c347bd53ed1f105d" +
			"6a0d30aba477e178889ab2ec55d558de" +
			"ab2630204336962b4db5b663b6902b89" +
			"e85b31bc6af50fc50accb3fb9b57b663" +
			"297031378db47896d7fbaf6c600add2c" +
			"67f936db037986db856eb49cf2db3f7d" +
			"a6d23650e438f1884041b013119e4c2a" +
			"e5af37cccdfb68660738b58b3c59d1c0" +
			"248437472aba1f35ca1fb90cd714aa9f" +
			"635534f49e7c5bba81c2b6b36fdee21c" +
			"a27e347f793d2ce944edb23c8c9b914b" +
			"e10335e350feb5070394b7a4a15c0ca1" +
			"20283568b7bfc254fe838b137a2147ce" +
			"7c113a3a4d65499d9e86b87dbcc7f03b" +
			"bd3a3ab1aa243ece5ba9bcf25f82836c" +
			"fe473b2d83e7a7201cd0b96a72451e86" +
			"3f6c3ba664a6d073d1f7b5ed990865d9" +
			"78bd3815d06094fc9a2aba5221c22d5a" +
			"b996389e3721e3af5f05beddc2875e0d" +
			"faeb39021ee27a41187cbb45ef40c3e7" +
			"3bc03989f9a30d12c54ba7d2141da8a8" +
			"75493e65776ef35f97debc2286cc4af9" +
			"b4623eee902f840c52f1b8ad658939ae" +
			"f71f3f72b9ec1de21588bd35484ea444" +
			"36343ff95ead6ab1d8afb1b2a303df1b" +
			"71e53c4aea6b2e3e9372be0d1bc99798" +
			"b0ce3cc10d2a596d565dba82f88ce4cf" +
			"f3b33d5d24e9c0831124bf1ad54b7925" +
			"32983dd6c3a8b7d0",
		mac: "abf3e651",
	},
}

// cipherKey decodes a test set's hex key.
func cipherKey(t *testing.T, s string) CipherKey {
	t.Helper()

	var k CipherKey

	copy(k[:], mustHex(t, s))

	return k
}

// integrityKey decodes a test set's hex key.
func integrityKey(t *testing.T, s string) IntegrityKey {
	t.Helper()

	var k IntegrityKey

	copy(k[:], mustHex(t, s))

	return k
}

// firstBitDiff reports the index of the first of the leading bits where a and b
// differ, or -1 when they agree over all of them. Annex C pads its payloads with
// zero bits past the declared length, which a keystream XOR does not reproduce,
// so a comparison has to stop where the test set does.
func firstBitDiff(a, b []byte, bits int) int {
	for i := range bits {
		octet, mask := i/8, byte(0x80>>(i%8))
		if octet >= len(a) || octet >= len(b) {
			return i
		}

		if a[octet]&mask != b[octet]&mask {
			return i
		}
	}

	return -1
}

// runIntegrityVectors checks one algorithm against its byte-aligned test sets,
// through the selector so the identifier-to-implementation mapping is covered too.
func runIntegrityVectors(t *testing.T, alg IntegrityAlgorithm, name string, vectors []integrityVector) {
	t.Helper()

	integ, err := IntegrityFor(alg)
	if err != nil {
		t.Fatalf("IntegrityFor(%s): %v", name, err)
	}

	for _, v := range vectors {
		msg := mustHex(t, v.msg)[:v.bits/8]

		mac, err := integ.MAC(integrityKey(t, v.key), v.count, v.bearer, v.dir, msg)
		if err != nil {
			t.Fatalf("TS 33.401 %s: %v", v.set, err)
		}

		if got, want := hex.EncodeToString(mac[:]), v.mac; got != want {
			t.Errorf("TS 33.401 %s: %s MAC = %s, want %s", v.set, name, got, want)
		}
	}
}

// TestFirstBitDiff guards the comparison the vector tests rely on: a helper that
// always reported equality would make every one of them pass vacuously.
func TestFirstBitDiff(t *testing.T) {
	cases := []struct {
		a, b []byte
		bits int
		want int
	}{
		{[]byte{0xFF}, []byte{0xFF}, 8, -1},
		{[]byte{0xFF}, []byte{0x7F}, 8, 0},
		{[]byte{0xFF}, []byte{0xFE}, 8, 7},
		{[]byte{0xFF}, []byte{0xFE}, 7, -1}, // the differing bit is past the length
		{[]byte{0xAA, 0x00}, []byte{0xAA, 0x80}, 16, 8},
		{[]byte{0xAA}, nil, 8, 0},
	}

	for _, tc := range cases {
		if got := firstBitDiff(tc.a, tc.b, tc.bits); got != tc.want {
			t.Errorf("firstBitDiff(%x, %x, %d) = %d, want %d", tc.a, tc.b, tc.bits, got, tc.want)
		}
	}
}
